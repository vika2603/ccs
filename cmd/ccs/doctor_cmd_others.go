//go:build !darwin

package main

import "github.com/vika2603/ccs/internal/doctor"

func newDoctorKeychainLister() doctor.KeychainLister { return nil }
