package operator

import (
	oauth "github.com/openshift/api/oauth/v1"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newOAuthClient( oauthSecret string, keycloakURL string, keycloakRealm string ) *oauth.OAuthClient {
	return &oauth.OAuthClient{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OAuthClient",
			APIVersion: oauth.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openshift-identity-provider",
			Labels: map[string]string{"app":"che"},
		},

		Secret: oauthSecret,
		RedirectURIs: []string{
				keycloakURL + "/auth/realms/" + keycloakRealm +"/broker/openshift-v3/endpoint",
		},
		GrantMethod: oauth.GrantHandlerPrompt,
	}

}

func CreateOAuthClient ( oauthSecret string, keycloakURL string, keycloakRealm string ) *oauth.OAuthClient {

	oauthClient := newOAuthClient( oauthSecret, keycloakURL, keycloakRealm)
	if err := sdk.Create(oauthClient); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("Failed to create oAuth client : %v", err)
		return nil
	}
	return oauthClient
}


