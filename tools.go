//go:build tools
// +build tools

// Package tools ensures indirect dependencies are tracked in go.mod.
package magescan

import (
	_ "github.com/charmbracelet/bubbles"
	_ "github.com/charmbracelet/bubbletea"
	_ "github.com/charmbracelet/lipgloss"
	_ "github.com/go-sql-driver/mysql"
)
