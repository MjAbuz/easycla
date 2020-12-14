// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"errors"
	"fmt"

	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"

	"github.com/google/go-github/v32/github"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

// GetInstallationRepositories returns list of repositories for github app installation
func GetInstallationRepositories(ctx context.Context, installationID int64) ([]*github.Repository, error) {
	f := logrus.Fields{
		"functionName":   "GetInstallationRepositories",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"installationID": installationID,
	}

	client, err := NewGithubAppClient(installationID)
	if err != nil {
		msg := fmt.Sprintf("unable to create a github client, error: %+v", err)
		log.WithFields(f).WithError(err).Warn(msg)
		return nil, errors.New(msg)
	}

	// Our response with all the repos
	var allRepos []*github.Repository

	// See pagination examples: https://godoc.org/github.com/google/go-github/github
	opts := &github.ListOptions{
		PerPage: 50,
	}

	for {
		repos, resp, err := client.Apps.ListRepos(ctx, opts)
		if err != nil {
			msg := fmt.Sprintf("error while getting repositories associated for installation, error: %+v", err)
			log.WithFields(f).WithError(err).Warn(msg)
			return nil, errors.New(msg)
		}

		log.WithFields(f).Debugf("fetched %d records...", len(repos))
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}
