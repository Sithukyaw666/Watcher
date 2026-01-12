package store

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	StatusSuccess    = "SUCCESS"
	StatusFailed     = "FAILED"
	StatusRolledBack = "ROLLED_BACK"
)

type Deployment struct {
	ID            string    `json:"id"`
	CommitHash    string    `json:"commit_hash"`
	Timestamp     time.Time `json:"timestamp"`
	Status        string    `json:"status"`
	CommitMessage string    `json:"commit_message"`
	CommitAuthor  string    `json:"commit_author"`
}

type Store struct {
	db *bolt.DB
}

func NewStore(dbPath string) (*Store, error) {
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{
		Timeout: 1 * time.Second,
	})

	if err != nil {
		return nil, fmt.Errorf("could not open bolt db: %w", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("deployment"))
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("count not set up bucket: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) AddDeployment(d Deployment) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("deployment"))

		data, err := json.Marshal(d)
		if err != nil {
			return fmt.Errorf("marshal failed: %w", err)
		}

		return b.Put([]byte(d.ID), data)
	})
}

func (s *Store) GetLastSuccessfulDeployment() (*Deployment, error) {
	var lastSuccess *Deployment

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("deployment"))

		c := b.Cursor()

		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			var d Deployment
			if err := json.Unmarshal(v, &d); err != nil {
				continue
			}
			if d.Status == StatusSuccess {
				lastSuccess = &d
				return nil
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	if lastSuccess == nil {
		return nil, fmt.Errorf("no successful deployment found")
	}
	return lastSuccess, nil
}

func (s *Store) GetAllDeployments() ([]Deployment, error) {
	var deployments []Deployment

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("deployment"))
		return b.ForEach(func(k, v []byte) error {
			var d Deployment
			if err := json.Unmarshal(v, &d); err != nil {
				return nil
			}
			deployments = append(deployments, d)
			return nil
		})
	})

	return deployments, err
}

func (s *Store) GetLastDeployment() (*Deployment, error) {
	var lastDeploy *Deployment

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("deployment"))
		c := b.Cursor()
		k, v := c.Last()
		if k == nil {
			return nil
		}
		var d Deployment
		if err := json.Unmarshal(v, &d); err != nil {
			return err
		}

		lastDeploy = &d
		return nil
	})

	if err != nil {
		return nil, err
	}
	if lastDeploy == nil {
		return nil, fmt.Errorf("no history found")
	}
	return lastDeploy, nil
}
