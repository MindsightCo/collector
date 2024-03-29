// Code generated by MockGen. DO NOT EDIT.
// Source: apiclient.go

// Package apiclient is a generated GoMock package.
package apiclient

import (
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
)

// MockTokenBuilder is a mock of TokenBuilder interface
type MockTokenBuilder struct {
	ctrl     *gomock.Controller
	recorder *MockTokenBuilderMockRecorder
}

// MockTokenBuilderMockRecorder is the mock recorder for MockTokenBuilder
type MockTokenBuilderMockRecorder struct {
	mock *MockTokenBuilder
}

// NewMockTokenBuilder creates a new mock instance
func NewMockTokenBuilder(ctrl *gomock.Controller) *MockTokenBuilder {
	mock := &MockTokenBuilder{ctrl: ctrl}
	mock.recorder = &MockTokenBuilderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockTokenBuilder) EXPECT() *MockTokenBuilderMockRecorder {
	return m.recorder
}

// GetAccessToken mocks base method
func (m *MockTokenBuilder) GetAccessToken() (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAccessToken")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAccessToken indicates an expected call of GetAccessToken
func (mr *MockTokenBuilderMockRecorder) GetAccessToken() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAccessToken", reflect.TypeOf((*MockTokenBuilder)(nil).GetAccessToken))
}
