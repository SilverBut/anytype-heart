// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/anyproto/anytype-heart/space/typeprovider (interfaces: SmartBlockTypeProvider)

// Package testMock is a generated GoMock package.
package testMock

import (
	reflect "reflect"

	app "github.com/anyproto/any-sync/app"
	smartblock "github.com/anyproto/anytype-heart/pkg/lib/core/smartblock"
	gomock "github.com/golang/mock/gomock"
)

// MockSmartBlockTypeProvider is a mock of SmartBlockTypeProvider interface.
type MockSmartBlockTypeProvider struct {
	ctrl     *gomock.Controller
	recorder *MockSmartBlockTypeProviderMockRecorder
}

// MockSmartBlockTypeProviderMockRecorder is the mock recorder for MockSmartBlockTypeProvider.
type MockSmartBlockTypeProviderMockRecorder struct {
	mock *MockSmartBlockTypeProvider
}

// NewMockSmartBlockTypeProvider creates a new mock instance.
func NewMockSmartBlockTypeProvider(ctrl *gomock.Controller) *MockSmartBlockTypeProvider {
	mock := &MockSmartBlockTypeProvider{ctrl: ctrl}
	mock.recorder = &MockSmartBlockTypeProviderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSmartBlockTypeProvider) EXPECT() *MockSmartBlockTypeProviderMockRecorder {
	return m.recorder
}

// Init mocks base method.
func (m *MockSmartBlockTypeProvider) Init(arg0 *app.App) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Init", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Init indicates an expected call of Init.
func (mr *MockSmartBlockTypeProviderMockRecorder) Init(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockSmartBlockTypeProvider)(nil).Init), arg0)
}

// Name mocks base method.
func (m *MockSmartBlockTypeProvider) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockSmartBlockTypeProviderMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockSmartBlockTypeProvider)(nil).Name))
}

// RegisterStaticType mocks base method.
func (m *MockSmartBlockTypeProvider) RegisterStaticType(arg0 string, arg1 smartblock.SmartBlockType) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "RegisterStaticType", arg0, arg1)
}

// RegisterStaticType indicates an expected call of RegisterStaticType.
func (mr *MockSmartBlockTypeProviderMockRecorder) RegisterStaticType(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RegisterStaticType", reflect.TypeOf((*MockSmartBlockTypeProvider)(nil).RegisterStaticType), arg0, arg1)
}

// Type mocks base method.
func (m *MockSmartBlockTypeProvider) Type(arg0 string) (smartblock.SmartBlockType, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Type", arg0)
	ret0, _ := ret[0].(smartblock.SmartBlockType)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Type indicates an expected call of Type.
func (mr *MockSmartBlockTypeProviderMockRecorder) Type(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Type", reflect.TypeOf((*MockSmartBlockTypeProvider)(nil).Type), arg0)
}