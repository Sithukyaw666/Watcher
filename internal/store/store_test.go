package store

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4/testutils/require"
	"github.com/go-openapi/testify/v2/assert"

	bolt "go.etcd.io/bbolt"
)

func TestNewStore(t *testing.T) {
	t.Run("it should create the test.db", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")
		s, err := NewStore(dbPath)

		defer s.Close()

		require.NoError(t, err)
	})

	t.Run("it should create the bucket if not exist", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")
		s, err := NewStore(dbPath)
		require.NoError(t, err)
		s.Close()

		db, err := bolt.Open(dbPath, 0600, &bolt.Options{ReadOnly: true})
		require.NoError(t, err)
		defer db.Close()

		db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(BucketName))
			assert.NotNil(t, b, "bucket should exist")
			return nil
		})
	})

	t.Run("it should not create with invalid dbPath", func(t *testing.T) {
		dbPath := "/path/not/exist"
		_, err := NewStore(dbPath)
		assert.NotNil(t, err, "expected error but got none")
	})
}

func TestAddDeployment(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.tb")
	s, err := NewStore(dbPath)
	defer s.Close()
	t.Run("it should add new deployment without error", func(t *testing.T) {
		data := Deployment{
			ID:            "1",
			CommitHash:    "hash",
			CommitMessage: "message",
			CommitAuthor:  "jondoe",
			Timestamp:     time.Now(),
			Status:        StatusSuccess,
		}

		require.NoError(t, err)
		err = s.AddDeployment(data)
		require.NoError(t, err)
	})
}

func TestGetLastDeployment(t *testing.T) {

	t.Run("it should return latest deployment", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")
		s, err := NewStore(dbPath)
		require.NoError(t, err)
		defer s.Close()
		for i := range 3 {
			s.AddDeployment(Deployment{
				ID:            fmt.Sprintf("%d", i),
				CommitHash:    fmt.Sprintf("hash-%d", i),
				CommitMessage: fmt.Sprintf("message-%d", i),
				CommitAuthor:  "jondoe",
				Timestamp:     time.Now(),
				Status:        StatusSuccess,
			})
		}
		d, err := s.GetLastDeployment()
		require.NoError(t, err)
		assert.NotNil(t, d, "should not be nil")
		assert.Equal(t, "2", d.ID, "should return the latest deployment")
	})

	t.Run("it should return error if store is empty", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")
		s, err := NewStore(dbPath)
		require.NoError(t, err)
		defer s.Close()

		_, err = s.GetLastDeployment()
		assert.NotNil(t, err, "expected error but got none")

	})
}

func TestGetLastDeployment_Empty(t *testing.T) {

}

func TestGetLasrSuccessfulDeployment(t *testing.T) {

	t.Run("it should return the latest successful deployment", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")
		s, err := NewStore(dbPath)
		require.NoError(t, err)
		defer s.Close()

		statuses := []string{StatusSuccess, StatusFailed, StatusSuccess, StatusFailed}
		for i, status := range statuses {
			s.AddDeployment(Deployment{
				ID:            fmt.Sprintf("%d", i),
				CommitHash:    fmt.Sprintf("hash-%d", i),
				CommitMessage: fmt.Sprintf("message-%d", i),
				CommitAuthor:  "jondoe",
				Timestamp:     time.Now(),
				Status:        status,
			})
		}
		d, err := s.GetLastSuccessfulDeployment()
		require.NoError(t, err)
		assert.NotNil(t, d, "should not return nil")

		assert.Equal(t, "2", d.ID, "should return third deployment")
	})

	t.Run("it should return an error if no successful deployment exist in store", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")
		s, err := NewStore(dbPath)
		require.NoError(t, err)
		defer s.Close()

		statuses := []string{StatusFailed}
		for i, status := range statuses {
			s.AddDeployment(Deployment{
				ID:            fmt.Sprintf("%d", i),
				CommitHash:    fmt.Sprintf("hash-%d", i),
				CommitMessage: fmt.Sprintf("message-%d", i),
				CommitAuthor:  "jondoe",
				Timestamp:     time.Now(),
				Status:        status,
			})
		}
		_, err = s.GetLastSuccessfulDeployment()
		assert.NotNil(t, err, "expected an error but got none")

	})
}

func TestGetAllDeployments(t *testing.T) {
	t.Run("it should return all the deployments", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")
		s, err := NewStore(dbPath)
		require.NoError(t, err)
		defer s.Close()
		for i := range 3 {
			s.AddDeployment(Deployment{
				ID:            fmt.Sprintf("%d", i),
				CommitHash:    fmt.Sprintf("hash-%d", i),
				CommitMessage: fmt.Sprintf("message-%d", i),
				CommitAuthor:  "jondoe",
				Timestamp:     time.Now(),
				Status:        StatusSuccess,
			})
		}
		d, err := s.GetAllDeployments()

		require.NoError(t, err)
		assert.NotNil(t, d, "should not return nil")
		assert.NotZero(t, len(d), "should not return empty deployment")
		assert.Equal(t, 3, len(d))
	})

	t.Run("it should return empty array if none existed in store", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")
		s, err := NewStore(dbPath)
		require.NoError(t, err)
		defer s.Close()
		d, err := s.GetAllDeployments()

		require.NoError(t, err)
		assert.Zero(t, len(d), "should return empty deployment array")

	})

}
