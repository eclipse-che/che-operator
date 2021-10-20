// Code generated by MockGen. DO NOT EDIT.
// Source: pkg/deploy/openshift-oauth/openshiftoauthuser.go

// Package openshiftoauth is a generated GoMock package.
package openshiftoauth

import (
	reflect "reflect"

	deploy "github.com/eclipse-che/che-operator/pkg/deploy"
	gomock "github.com/golang/mock/gomock"
)

// MockIOpenShiftOAuthUser is a mock of IOpenShiftOAuthUser interface
type MockIOpenShiftOAuthUser struct {
	ctrl     *gomock.Controller
	recorder *MockIOpenShiftOAuthUserMockRecorder
}

// MockIOpenShiftOAuthUserMockRecorder is the mock recorder for MockIOpenShiftOAuthUser
type MockIOpenShiftOAuthUserMockRecorder struct {
	mock *MockIOpenShiftOAuthUser
}

// NewMockIOpenShiftOAuthUser creates a new mock instance
func NewMockIOpenShiftOAuthUser(ctrl *gomock.Controller) *MockIOpenShiftOAuthUser {
	mock := &MockIOpenShiftOAuthUser{ctrl: ctrl}
	mock.recorder = &MockIOpenShiftOAuthUserMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockIOpenShiftOAuthUser) EXPECT() *MockIOpenShiftOAuthUserMockRecorder {
	return m.recorder
}

// Create mocks base method
func (m *MockIOpenShiftOAuthUser) Create(ctx *deploy.DeployContext) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Create", ctx)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Create indicates an expected call of Create
func (mr *MockIOpenShiftOAuthUserMockRecorder) Create(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockIOpenShiftOAuthUser)(nil).Create), ctx)
}

// Delete mocks base method
func (m *MockIOpenShiftOAuthUser) Delete(ctx *deploy.DeployContext) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete
func (mr *MockIOpenShiftOAuthUserMockRecorder) Delete(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockIOpenShiftOAuthUser)(nil).Delete), ctx)
}
