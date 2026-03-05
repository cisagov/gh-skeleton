package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// RESTClient abstracts the GitHub REST API.
type RESTClient interface {
	Get(path string, resp interface{}) error
	Patch(path string, body io.Reader, resp interface{}) error
	Post(path string, body io.Reader, resp interface{}) error
	Put(path string, body io.Reader, resp interface{}) error
}

func configureRepoOptions(destRepo, destOrg, branch string, client RESTClient) error {
	logInfo("Configuring repository options for %s/%s.", destOrg, destRepo)

	payload := map[string]interface{}{
		"allow_auto_merge":       false,
		"allow_merge_commit":     true,
		"allow_rebase_merge":     true,
		"allow_squash_merge":     false,
		"default_branch":         branch,
		"delete_branch_on_merge": true,
		"private":                false,
		"visibility":             "public",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("json marshal failed: %w", err)
	}

	var resp interface{}
	if err := client.Patch("repos/"+destOrg+"/"+destRepo, bytes.NewReader(data), &resp); err != nil {
		return fmt.Errorf("patch repo options failed: %w", err)
	}
	return nil
}

func configureBranchProtection(destRepo, destOrg, branch string, client RESTClient) error {
	logInfo("Determining organization type for %s.", destOrg)

	var orgResp struct{}
	if err := client.Get("orgs/"+destOrg, &orgResp); err == nil {
		logOk("Organization %s is a true organization.", destOrg)
		return configureOrgBranchProtection(destRepo, destOrg, branch, client)
	}
	logOk("Organization %s is a personal account.", destOrg)
	return configureUserBranchProtection(destRepo, destOrg, branch, client)
}

func configureOrgBranchProtection(destRepo, destOrg, branch string, client RESTClient) error {
	logInfo("Configuring organization branch protection for %s/%s/%s.", destOrg, destRepo, branch)

	payload := map[string]interface{}{
		"enforce_admins":                   true,
		"required_conversation_resolution": true,
		"required_pull_request_reviews": map[string]interface{}{
			"dismiss_stale_reviews": false,
			"dismissal_restrictions": map[string]interface{}{
				"teams": []interface{}{},
				"users": []interface{}{},
			},
			"require_code_owner_reviews":      true,
			"required_approving_review_count": 2,
		},
		"required_status_checks": map[string]interface{}{
			"checks": []interface{}{
				map[string]interface{}{"context": "lint"},
			},
			"strict": true,
		},
		"restrictions": map[string]interface{}{
			"apps":  []interface{}{},
			"teams": []interface{}{},
			"users": []interface{}{},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("json marshal failed: %w", err)
	}

	var resp interface{}
	path := "repos/" + destOrg + "/" + destRepo + "/branches/" + branch + "/protection"
	if err := client.Put(path, bytes.NewReader(data), &resp); err != nil {
		return fmt.Errorf("put branch protection failed: %w", err)
	}
	return nil
}

func configureUserBranchProtection(destRepo, destOrg, branch string, client RESTClient) error {
	logInfo("Configuring user branch protection for %s/%s/%s.", destOrg, destRepo, branch)

	payload := map[string]interface{}{
		"enforce_admins":                   true,
		"required_conversation_resolution": true,
		"required_pull_request_reviews": map[string]interface{}{
			"dismiss_stale_reviews":           false,
			"require_code_owner_reviews":      true,
			"required_approving_review_count": 2,
		},
		"required_status_checks": map[string]interface{}{
			"checks": []interface{}{
				map[string]interface{}{"context": "lint"},
			},
			"strict": true,
		},
		"restrictions": nil,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("json marshal failed: %w", err)
	}

	var resp interface{}
	path := "repos/" + destOrg + "/" + destRepo + "/branches/" + branch + "/protection"
	if err := client.Put(path, bytes.NewReader(data), &resp); err != nil {
		return fmt.Errorf("put branch protection failed: %w", err)
	}
	return nil
}
