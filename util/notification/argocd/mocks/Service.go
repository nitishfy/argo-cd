// Code generated by mockery v2.52.4. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	shared "github.com/argoproj/argo-cd/v3/util/notification/expression/shared"

	v1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// Service is an autogenerated mock type for the Service type
type Service struct {
	mock.Mock
}

// GetAppDetails provides a mock function with given fields: ctx, app
func (_m *Service) GetAppDetails(ctx context.Context, app *v1alpha1.Application) (*shared.AppDetail, error) {
	ret := _m.Called(ctx, app)

	if len(ret) == 0 {
		panic("no return value specified for GetAppDetails")
	}

	var r0 *shared.AppDetail
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.Application) (*shared.AppDetail, error)); ok {
		return rf(ctx, app)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.Application) *shared.AppDetail); ok {
		r0 = rf(ctx, app)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*shared.AppDetail)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1alpha1.Application) error); ok {
		r1 = rf(ctx, app)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetCommitMetadata provides a mock function with given fields: ctx, repoURL, commitSHA, project
func (_m *Service) GetCommitMetadata(ctx context.Context, repoURL string, commitSHA string, project string) (*shared.CommitMetadata, error) {
	ret := _m.Called(ctx, repoURL, commitSHA, project)

	if len(ret) == 0 {
		panic("no return value specified for GetCommitMetadata")
	}

	var r0 *shared.CommitMetadata
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string) (*shared.CommitMetadata, error)); ok {
		return rf(ctx, repoURL, commitSHA, project)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string) *shared.CommitMetadata); ok {
		r0 = rf(ctx, repoURL, commitSHA, project)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*shared.CommitMetadata)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, string) error); ok {
		r1 = rf(ctx, repoURL, commitSHA, project)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewService creates a new instance of Service. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewService(t interface {
	mock.TestingT
	Cleanup(func())
}) *Service {
	mock := &Service{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
