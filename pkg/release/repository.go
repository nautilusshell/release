/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package release

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"k8s.io/release/pkg/git"
)

// Repo is a wrapper around a kubernetes/release repository
type Repo struct {
	repo Repository
}

// NewRepo creates a new release repository
func NewRepo() *Repo {
	return &Repo{}
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate . Repository
type Repository interface {
	Describe(opts *git.DescribeOptions) (string, error)
	CurrentBranch() (branch string, err error)
	Remotes() (res []*git.Remote, err error)
}

// Open assumes the current working directory as repository root and tries to
// open it
func (r *Repo) Open() error {
	dir, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "getting current working directory")
	}
	repo, err := git.OpenRepo(dir)
	if err != nil {
		return errors.Wrap(err, "opening release repository")
	}
	r.repo = repo
	return nil
}

// SetRepo can be used to set the internal repository implementation
func (r *Repo) SetRepo(repo Repository) {
	r.repo = repo
}

// GetTag returns the tag from the current repository
func (r *Repo) GetTag() (string, error) {
	describeOutput, err := r.repo.Describe(
		git.NewDescribeOptions().
			WithAlways().
			WithDirty().
			WithTags(),
	)
	if err != nil {
		return "", errors.Wrap(err, "running git describe")
	}
	t := time.Now().Format("20060102")
	return fmt.Sprintf("%s-%s", describeOutput, t), nil
}

// CheckState verifies that the repository is in the requested state
func (r *Repo) CheckState(expOrg, expRepo, expBranch string) error {
	logrus.Info("Verifying repository state")

	// Verify the branch
	branch, err := r.repo.CurrentBranch()
	if err != nil {
		return errors.Wrap(err, "retrieving current branch")
	}
	if branch != expBranch {
		return errors.Errorf("branch %s expected but got %s", expBranch, branch)
	}
	logrus.Infof("Found matching branch %s", expBranch)

	// Verify the remote
	remotes, err := r.repo.Remotes()
	if err != nil {
		return errors.Wrap(err, "retrieving repository remotes")
	}
	var foundRemote *git.Remote
	for _, remote := range remotes {
		for _, url := range remote.URLs() {
			if strings.Contains(url, filepath.Join(expOrg, expRepo)) {
				foundRemote = remote
				break
			}
		}
		if foundRemote != nil {
			break
		}
	}
	if foundRemote == nil {
		return errors.Errorf(
			"unable to find remote matching organization %s and repository %s",
			expOrg, expRepo,
		)
	}
	logrus.Infof(
		"Found matching organization %s and repository %s in remote: %s (%s)",
		expOrg,
		expRepo,
		foundRemote.Name(),
		strings.Join(foundRemote.URLs(), ", "),
	)

	// TODO: verify if the branch contains the remote commit via "ls-remote" if
	// the logic in the git package is implemented (follow-up)
	return nil
}
