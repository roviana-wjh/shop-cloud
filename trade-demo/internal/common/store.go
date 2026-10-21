package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Store[T any] struct {
	mu   sync.Mutex
	Path string
	Data T
}

func OpenStore[T any](path string, initial T) (*Store[T], error) {
	s := &Store[T]{Path: path, Data: initial}
	b, err := os.ReadFile(path)
	if err == nil {
		if err = json.Unmarshal(b, &s.Data); err != nil {
			return nil, fmt.Errorf("load %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

func (s *Store[T]) Read(fn func(T)) { s.mu.Lock(); defer s.mu.Unlock(); fn(s.Data) }

func (s *Store[T]) Update(fn func(*T) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	backup, err := json.Marshal(s.Data)
	if err != nil {
		return err
	}
	rollback := func() { var restored T; _ = json.Unmarshal(backup, &restored); s.Data = restored }
	if err = fn(&s.Data); err != nil {
		rollback()
		return err
	}
	b, err := json.MarshalIndent(s.Data, "", "  ")
	if err != nil {
		rollback()
		return err
	}
	if err = os.MkdirAll(filepath.Dir(s.Path), 0755); err != nil {
		rollback()
		return err
	}
	tmp := s.Path + ".tmp"
	if err = os.WriteFile(tmp, b, 0600); err != nil {
		rollback()
		return err
	}
	if err = os.Rename(tmp, s.Path); err != nil {
		rollback()
		return err
	}
	return nil
}
