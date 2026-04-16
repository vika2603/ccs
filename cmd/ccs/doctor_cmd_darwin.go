//go:build darwin

package main

import "github.com/vika2603/ccs/internal/doctor"

func newDoctorKeychainLister() doctor.KeychainLister {
	return keychainListerFunc(dumpKeychainServices)
}

type keychainListerFunc func() ([]string, error)

func (f keychainListerFunc) List() ([]string, error) { return f() }
