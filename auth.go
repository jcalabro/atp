package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/jcalabro/atmos/xrpc"
)

type sessionFile struct {
	Host       string `json:"host"`
	AccessJwt  string `json:"accessJwt"`
	RefreshJwt string `json:"refreshJwt"`
	Handle     string `json:"handle"`
	DID        string `json:"did"`
}

func sessionPath() string {
	return filepath.Join(xdg.ConfigHome, "atp", "session.json")
}

func loadSession() (*sessionFile, error) {
	data, err := os.ReadFile(sessionPath())
	if err != nil {
		return nil, err
	}
	var s sessionFile
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func saveSession(s *sessionFile) error {
	dir := filepath.Dir(sessionPath())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sessionPath(), data, 0o600)
}

func deleteSession() error {
	return os.Remove(sessionPath())
}

func clientFromSession() (*xrpc.Client, *sessionFile, error) {
	sess, err := loadSession()
	if err != nil {
		return nil, nil, err
	}
	client := &xrpc.Client{Host: sess.Host}
	client.SetAuth(&xrpc.AuthInfo{
		AccessJwt:  sess.AccessJwt,
		RefreshJwt: sess.RefreshJwt,
		Handle:     sess.Handle,
		DID:        sess.DID,
	})
	return client, sess, nil
}
