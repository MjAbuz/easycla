// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

// Code generated by MockGen. DO NOT EDIT.
// Source: protected_branch.go

// Package github is a generated GoMock package.
package github

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	github "github.com/google/go-github/v33/github"
)

// MockRepositories is a mock of Repositories interface
type MockRepositories struct {
	ctrl     *gomock.Controller
	recorder *MockRepositoriesMockRecorder
}

// MockRepositoriesMockRecorder is the mock recorder for MockRepositories
type MockRepositoriesMockRecorder struct {
	mock *MockRepositories
}

// NewMockRepositories creates a new mock instance
func NewMockRepositories(ctrl *gomock.Controller) *MockRepositories {
	mock := &MockRepositories{ctrl: ctrl}
	mock.recorder = &MockRepositoriesMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockRepositories) EXPECT() *MockRepositoriesMockRecorder {
	return m.recorder
}

// ListByOrg mocks base method
func (m *MockRepositories) ListByOrg(ctx context.Context, org string, opt *github.RepositoryListByOrgOptions) ([]*github.Repository, *github.Response, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListByOrg", ctx, org, opt)
	ret0, _ := ret[0].([]*github.Repository)
	ret1, _ := ret[1].(*github.Response)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ListByOrg indicates an expected call of ListByOrg
func (mr *MockRepositoriesMockRecorder) ListByOrg(ctx, org, opt interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListByOrg", reflect.TypeOf((*MockRepositories)(nil).ListByOrg), ctx, org, opt)
}

// Get mocks base method
func (m *MockRepositories) Get(ctx context.Context, owner, repo string) (*github.Repository, *github.Response, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", ctx, owner, repo)
	ret0, _ := ret[0].(*github.Repository)
	ret1, _ := ret[1].(*github.Response)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Get indicates an expected call of Get
func (mr *MockRepositoriesMockRecorder) Get(ctx, owner, repo interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockRepositories)(nil).Get), ctx, owner, repo)
}

// GetBranchProtection mocks base method
func (m *MockRepositories) GetBranchProtection(ctx context.Context, owner, repo, branch string) (*github.Protection, *github.Response, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBranchProtection", ctx, owner, repo, branch)
	ret0, _ := ret[0].(*github.Protection)
	ret1, _ := ret[1].(*github.Response)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetBranchProtection indicates an expected call of GetBranchProtection
func (mr *MockRepositoriesMockRecorder) GetBranchProtection(ctx, owner, repo, branch interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBranchProtection", reflect.TypeOf((*MockRepositories)(nil).GetBranchProtection), ctx, owner, repo, branch)
}

// UpdateBranchProtection mocks base method
func (m *MockRepositories) UpdateBranchProtection(ctx context.Context, owner, repo, branch string, preq *github.ProtectionRequest) (*github.Protection, *github.Response, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateBranchProtection", ctx, owner, repo, branch, preq)
	ret0, _ := ret[0].(*github.Protection)
	ret1, _ := ret[1].(*github.Response)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// UpdateBranchProtection indicates an expected call of UpdateBranchProtection
func (mr *MockRepositoriesMockRecorder) UpdateBranchProtection(ctx, owner, repo, branch, preq interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateBranchProtection", reflect.TypeOf((*MockRepositories)(nil).UpdateBranchProtection), ctx, owner, repo, branch, preq)
}
