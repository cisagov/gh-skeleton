package main

import (
	"fmt"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
)

// mockGraphQLClient is a manual mock for GraphQLClient.
type mockGraphQLClient struct {
	ctrl     *gomock.Controller
	response interface{}
	err      error
}

func newMockGraphQLClient(ctrl *gomock.Controller) *mockGraphQLClient {
	return &mockGraphQLClient{ctrl: ctrl}
}

func (m *mockGraphQLClient) Do(query string, variables map[string]interface{}, response interface{}) error {
	if m.err != nil {
		return m.err
	}
	if m.response != nil {
		src, ok := m.response.(*searchResult)
		if ok {
			dst, ok2 := response.(*searchResult)
			if ok2 {
				*dst = *src
			}
		}
	}
	return nil
}

func TestListSkeletons(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := newMockGraphQLClient(ctrl)
	mock.response = &searchResult{
		Search: struct {
			RepositoryCount int
			Edges           []struct {
				Node struct {
					Name        string
					Description string
				}
			}
		}{
			RepositoryCount: 2,
			Edges: []struct {
				Node struct {
					Name        string
					Description string
				}
			}{
				{Node: struct {
					Name        string
					Description string
				}{Name: "skeleton-python-library", Description: "A skeleton Python library."}},
				{Node: struct {
					Name        string
					Description string
				}{Name: "skeleton-generic", Description: "A generic skeleton."}},
			},
		},
	}

	// Capture stdout by redirecting - just call and check no error.
	err := listSkeletons("cisagov", mock)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestListSkeletonsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := newMockGraphQLClient(ctrl)
	mock.err = fmt.Errorf("API error")

	err := listSkeletons("cisagov", mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "graphql query failed") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
