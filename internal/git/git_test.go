package ops_git

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-jose/go-jose/v4/testutils/require"
	"github.com/go-openapi/testify/v2/assert"
	"github.com/sithukyaw666/watcher/internal/model"
	"github.com/sithukyaw666/watcher/internal/store"
)

type mockStore struct {
	lastDeployment *store.Deployment
	err            error
}

func (m *mockStore) GetLastDeployment() (*store.Deployment, error) {
	return m.lastDeployment, m.err
}

func TestCloneOrFetchRepo(t *testing.T) {

	logger := slog.New(slog.DiscardHandler)
	repoPath := filepath.Join(t.TempDir(), "test.git")
	repo, err := git.PlainInit(repoPath, false)

	require.NoError(t, err)
	w, _ := repo.Worktree()
	os.WriteFile(filepath.Join(repoPath, "docker-compose.yaml"), []byte("services:\n  web:\n    image: nginx\n"), 0644)
	w.Add("docker-compose.yaml")
	_, err = w.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com", When: time.Now()},
	})
	require.NoError(t, err)
	config := model.Config{
		DeploymentDir: t.TempDir(),
		RepoURL:       repoPath,
		ComposeFile:   "docker-compose.yaml",
		TargetBranch:  "master",
	}
	mock := &mockStore{
		lastDeployment: &store.Deployment{
			CommitHash: "hash",
			Status:     store.StatusFailed,
		},
	}

	t.Run("it should clone the repo and return GitService for init", func(t *testing.T) {
		gitService, err := NewGitService(config, logger)
		require.NoError(t, err)
		require.NoError(t, err)
		assert.NotNil(t, gitService)
	})

	t.Run("it should call fetch and return false on no change", func(t *testing.T) {
		gitService, err := NewGitService(config, logger)
		require.NoError(t, err)
		isUpdated, err := gitService.FetchRepo(config, mock)
		require.NoError(t, err)
		assert.False(t, isUpdated)
	})
	t.Run("it should fetch and return true on new commit", func(t *testing.T) {
		os.WriteFile(filepath.Join(repoPath, "readme.md"), []byte("helloworld"), 0644)
		w.Add("readme.md")
		_, err := w.Commit("added readme", &git.CommitOptions{
			Author: &object.Signature{Name: "test", Email: "test@test.com", When: time.Now()},
		})
		require.NoError(t, err)
		gitService, err := NewGitService(config, logger)
		require.NoError(t, err)
		isUpdated, err := gitService.FetchRepo(config, mock)
		require.NoError(t, err)
		assert.True(t, isUpdated)
	})

	t.Run("it should skip and return false on same fail config", func(t *testing.T) {
		head, err := repo.Head()
		require.NoError(t, err)
		mock := &mockStore{
			lastDeployment: &store.Deployment{
				CommitHash: head.Hash().String(),
				Status:     store.StatusFailed,
			},
		}
		gitService, err := NewGitService(config, logger)
		require.NoError(t, err)
		isUpdated, err := gitService.FetchRepo(config, mock)
		require.NoError(t, err)
		assert.False(t, isUpdated)
	})

}

func TestGetCurrentHash(t *testing.T) {
	t.Run("it should return the correct hash", func(t *testing.T) {
		logger := slog.New(slog.DiscardHandler)
		repoPath := filepath.Join(t.TempDir(), "test.git")
		repo, err := git.PlainInit(repoPath, false)

		require.NoError(t, err)
		w, _ := repo.Worktree()
		os.WriteFile(filepath.Join(repoPath, "docker-compose.yaml"), []byte("services:\n  web:\n    image: nginx\n"), 0644)
		w.Add("docker-compose.yaml")
		_, err = w.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{Name: "test", Email: "test@test.com", When: time.Now()},
		})
		headRef, err := repo.Head()
		require.NoError(t, err)

		remoteCommitHash := headRef.Hash().String()
		require.NoError(t, err)
		config := model.Config{
			DeploymentDir: t.TempDir(),
			RepoURL:       repoPath,
			ComposeFile:   "docker-compose.yaml",
			TargetBranch:  "master",
		}
		mock := &mockStore{
			lastDeployment: &store.Deployment{
				CommitHash: "hash",
				Status:     store.StatusFailed,
			},
		}
		gitService, err := NewGitService(config, logger)
		require.NoError(t, err)

		_, err = gitService.FetchRepo(config, mock)
		require.NoError(t, err)

		currentHash, err := gitService.GetCurrentHash(config)
		require.NoError(t, err)
		assert.Equal(t, remoteCommitHash, currentHash)
	})

}

func TestCheckoutHash(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	repoPath := filepath.Join(t.TempDir(), "test.git")
	repo, err := git.PlainInit(repoPath, false)

	require.NoError(t, err)
	w, _ := repo.Worktree()
	os.WriteFile(filepath.Join(repoPath, "docker-compose.yaml"), []byte("services:\n  web:\n    image: nginx\n"), 0644)
	w.Add("docker-compose.yaml")
	_, err = w.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com", When: time.Now()},
	})
	headRef, err := repo.Head()
	require.NoError(t, err)
	firstCommitHash := headRef.Hash().String()
	os.WriteFile(filepath.Join(repoPath, "readme.md"), []byte("helloworld"), 0644)
	w.Add("readme.md")
	_, err = w.Commit("second commit", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com", When: time.Now()},
	})
	require.NoError(t, err)
	config := model.Config{
		DeploymentDir: t.TempDir(),
		RepoURL:       repoPath,
		ComposeFile:   "docker-compose.yaml",
		TargetBranch:  "master",
	}
	mock := &mockStore{
		lastDeployment: &store.Deployment{
			CommitHash: "hash",
			Status:     store.StatusFailed,
		},
	}
	gitService, err := NewGitService(config, logger)
	require.NoError(t, err)
	_, err = gitService.FetchRepo(config, mock)
	require.NoError(t, err)
	t.Run("it should checkout to targeted hash", func(t *testing.T) {

		err = gitService.CheckoutHash(config, firstCommitHash)
		require.NoError(t, err)
		currenthash, err := gitService.GetCurrentHash(config)
		require.NoError(t, err)
		assert.Equal(t, firstCommitHash, currenthash)

	})

	t.Run("it should return error on invalid hash", func(t *testing.T) {
		err := gitService.CheckoutHash(config, "not-existed-hash")
		assert.NotNil(t, err, "expected error but got none")
	})
	t.Run("it should return error on valid hash format but non existence", func(t *testing.T) {
		hash := "a64396050c5bb118f935296043777a71c90f7905"
		err := gitService.CheckoutHash(config, hash)
		assert.NotNil(t, err, "expected error but got none")
	})

}

func TestGetCommitInfo(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	repoPath := filepath.Join(t.TempDir(), "test.git")
	repo, err := git.PlainInit(repoPath, false)

	require.NoError(t, err)
	w, _ := repo.Worktree()
	commitMsg := "initial commit"
	authorName := "test"
	os.WriteFile(filepath.Join(repoPath, "docker-compose.yaml"), []byte("services:\n  web:\n    image: nginx\n"), 0644)
	w.Add("docker-compose.yaml")
	_, err = w.Commit(commitMsg, &git.CommitOptions{
		Author: &object.Signature{Name: authorName, Email: "test@test.com", When: time.Now()},
	})
	require.NoError(t, err)
	config := model.Config{
		DeploymentDir: t.TempDir(),
		RepoURL:       repoPath,
		ComposeFile:   "docker-compose.yaml",
		TargetBranch:  "master",
	}
	mock := &mockStore{
		lastDeployment: &store.Deployment{
			CommitHash: "hash",
			Status:     store.StatusFailed,
		},
	}
	gitService, err := NewGitService(config, logger)
	require.NoError(t, err)
	_, err = gitService.FetchRepo(config, mock)
	require.NoError(t, err)

	t.Run("it should return commit info ", func(t *testing.T) {
		hash, err := gitService.GetCurrentHash(config)
		assert.NoError(t, err)
		msg, author, err := gitService.GetCommitInfo(config, hash)
		assert.NoError(t, err)
		assert.Equal(t, authorName, author)
		assert.Equal(t, commitMsg, msg)
	})
	t.Run("it should return error on invalid hash", func(t *testing.T) {
		_, _, err = gitService.GetCommitInfo(config, "invalid")
		assert.NotNil(t, err, "expected error but got none")
	})
	t.Run("it should return error on valid hash format but non existence", func(t *testing.T) {
		hash := "a64396050c5bb118f935296043777a71c90f7905"
		_, _, err := gitService.GetCommitInfo(config, hash)
		assert.NotNil(t, err, "expected error but got none")
	})

}
