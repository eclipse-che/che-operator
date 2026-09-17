//
// Copyright (c) 2019-2026 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package consolelink

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	consolev1 "github.com/openshift/api/console/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/idna"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ConsoleLinkFinalizerName   = "consolelink.finalizers.che.eclipse.org"
	CheDashboardRedirectURLEnv = "CHE_DASHBOARD_REDIRECT_URL"
)

var consoleLinkDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(consolev1.ConsoleLink{}, "TypeMeta", "ObjectMeta"),
}

type ConsoleLinkReconciler struct {
	reconciler.Reconcilable
}

func NewConsoleLinkReconciler() *ConsoleLinkReconciler {
	return &ConsoleLinkReconciler{}
}

func (c *ConsoleLinkReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	done, err := c.syncConsoleLink(ctx)
	if !done {
		return reconcile.Result{RequeueAfter: time.Second}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (c *ConsoleLinkReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	if err := deploy.DeleteObjectWithFinalizer(ctx, client.ObjectKey{Name: defaults.GetConsoleLinkName()}, &consolev1.ConsoleLink{}, ConsoleLinkFinalizerName); err != nil {
		logrus.Errorf("Error deleting finalizer: %v", err)
		return false
	}
	return true
}

func (c *ConsoleLinkReconciler) syncConsoleLink(ctx *chetypes.DeployContext) (bool, error) {
	if err := deploy.AppendFinalizer(ctx, ConsoleLinkFinalizerName); err != nil {
		return false, err
	}

	consoleLinkSpec := c.getConsoleLinkSpec(ctx)
	return deploy.Sync(ctx, consoleLinkSpec, consoleLinkDiffOpts)
}

func (c *ConsoleLinkReconciler) getConsoleLinkSpec(ctx *chetypes.DeployContext) *consolev1.ConsoleLink {
	href := "https://" + ctx.CheHost
	if redirectURL := getDashboardRedirectURL(ctx); redirectURL != "" {
		href = redirectURL
	}

	consoleLink := &consolev1.ConsoleLink{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConsoleLink",
			APIVersion: consolev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: defaults.GetConsoleLinkName(),
		},
		Spec: consolev1.ConsoleLinkSpec{
			Link: consolev1.Link{
				Href: href,
				Text: defaults.GetConsoleLinkDisplayName()},
			Location: consolev1.ApplicationMenu,
			ApplicationMenu: &consolev1.ApplicationMenuSpec{
				Section:  defaults.GetConsoleLinkSection(),
				ImageURL: fmt.Sprintf("https://%s%s", ctx.CheHost, defaults.GetConsoleLinkImage()),
			},
		},
	}

	return consoleLink
}

func getDashboardOverrideContainer(ctx *chetypes.DeployContext) *chev2.Container {
	if ctx == nil || ctx.CheCluster == nil || ctx.CheCluster.Spec.Components.Dashboard.Deployment == nil {
		return nil
	}
	containers := ctx.CheCluster.Spec.Components.Dashboard.Deployment.Containers
	if len(containers) == 0 {
		return nil
	}

	// The generated dashboard Deployment has one container (<flavor>-dashboard).
	// OverrideDeployment therefore applies only the first override, regardless of its
	// name or the number of override entries. Later entries do not create sidecars.
	return &containers[0]
}

func normalizeHostPort(scheme, hostPort string) string {
	host := hostPort
	port := ""
	if h, p, err := net.SplitHostPort(hostPort); err == nil {
		host = h
		port = p
		if number, err := strconv.Atoi(port); err == nil {
			port = strconv.Itoa(number)
		}
	}
	host = strings.Trim(host, "[]")
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	} else if ascii, err := idna.Lookup.ToASCII(host); err == nil {
		host = ascii
	}
	if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		port = ""
	}
	if port != "" {
		return strings.ToLower(net.JoinHostPort(host, port))
	}
	return strings.ToLower(host)
}

func isSelfRedirectLoop(u *url.URL, cheHost string) bool {
	if cheHost == "" || u == nil {
		return false
	}
	uHost := normalizeHostPort(u.Scheme, u.Host)
	cheHostNormalized := normalizeHostPort("https", cheHost)

	if u.Scheme == "https" && strings.EqualFold(uHost, cheHostNormalized) {
		// Browsers resolve literal and percent-encoded dot segments before navigating.
		segments := []string{}
		for _, segment := range strings.Split(u.EscapedPath(), "/") {
			switch strings.ReplaceAll(strings.ToLower(segment), "%2e", ".") {
			case ".":
				continue
			case "..":
				if len(segments) > 1 {
					segments = segments[:len(segments)-1]
				}
			default:
				segments = append(segments, segment)
			}
		}
		cleanPath := strings.TrimRight(strings.Join(segments, "/"), "/")
		if cleanPath == "" || cleanPath == "/index.html" {
			return true
		}
	}
	return false
}

func getDashboardRedirectURL(ctx *chetypes.DeployContext) string {
	container := getDashboardOverrideContainer(ctx)
	if container == nil {
		return ""
	}

	// In deploy.OverrideContainer, env overrides are applied sequentially,
	// so the last occurrence of an env variable with the same name takes precedence.
	var effectiveEnv *corev1.EnvVar
	for i := range container.Env {
		if container.Env[i].Name == CheDashboardRedirectURLEnv {
			effectiveEnv = &container.Env[i]
		}
	}
	if effectiveEnv == nil {
		return ""
	}

	// ValueFrom cannot be resolved safely or synchronously at reconciliation time.
	// Fall back to default CheHost.
	if effectiveEnv.ValueFrom != nil || effectiveEnv.Value == "" {
		return ""
	}

	// Match JavaScript String.trim, including BOM but excluding NEXT LINE.
	val := strings.TrimFunc(effectiveEnv.Value, func(r rune) bool {
		return (unicode.IsSpace(r) && r != '\u0085') || r == '\ufeff'
	})
	// Kubernetes expands $(VAR) in literal values; unresolved expansion must not
	// become a ConsoleLink that differs from the dashboard's effective environment.
	if val == "" || strings.Contains(val, "\\") || strings.Contains(val, "$(") || strings.Contains(val, "$$") ||
		strings.IndexFunc(val, func(r rune) bool {
			return unicode.IsControl(r) || unicode.IsSpace(r) || r == '\ufeff'
		}) >= 0 {
		return ""
	}

	u, err := url.Parse(val)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return ""
	}
	if port := u.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 0 || number > 65535 {
			return ""
		}
	}
	if strings.ContainsAny(u.Hostname(), ":[]") && net.ParseIP(u.Hostname()) == nil {
		return ""
	}
	if strings.HasPrefix(u.Host, "[") && !strings.Contains(u.Hostname(), ":") {
		return ""
	}
	hostname := u.Hostname()
	if net.ParseIP(hostname) == nil {
		hostname, err = idna.Lookup.ToASCII(hostname)
		if err != nil || strings.ContainsAny(hostname, "<>^|%") {
			return ""
		}
	}
	// Numeric hosts must be valid IPv4 addresses. Go's URL parser otherwise accepts
	// values such as 999.999.999.999 that browsers reject.
	labels := strings.Split(strings.TrimRight(hostname, "."), ".")
	last := labels[len(labels)-1]
	if _, err := strconv.ParseUint(last, 0, 64); err == nil || strings.Trim(last, "0123456789") == "" {
		if net.ParseIP(hostname) == nil {
			return ""
		}
	}

	// Reject navigation back to an entry point that runs the root preload script.
	if isSelfRedirectLoop(u, ctx.CheHost) {
		return ""
	}

	return val
}
