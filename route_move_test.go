package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHasAncestor(t *testing.T) {
	require.True(t, hasAncestor("", ""))
	require.False(t, hasAncestor("/a/b/c", "/a/b"))
	require.False(t, hasAncestor("/foo/bar", "/a/b/c"))
	require.True(t, hasAncestor("/foo/bar", "/foo/bar/c"))
	require.True(t, hasAncestor("/", "/a/b/c"))
	require.False(t, hasAncestor("/a/b", ""))
	require.True(t, hasAncestor("", "/a/b/c"))
	require.False(t, hasAncestor("a//b", "a"))
	require.True(t, hasAncestor("a/", "a/b/"))
	require.True(t, hasAncestor("//////", "/a/b/c"))
	require.True(t, hasAncestor("/a//b///c", "////a///b/c/d/e"))
}
