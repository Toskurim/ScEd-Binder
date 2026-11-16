//go:build !windows

package main

func pickFolder(title string) (string, error) { return "", nil }
