//go:build !android && !ios

package main

// onPhone is false on computers, where the app is a window of its own.
const onPhone = false
