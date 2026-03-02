package pruner

import (
	"strings"
	"testing"
)

// --- 7.4 Segmenter Tests ---

func TestSegmenter_Go(t *testing.T) {
	code := `package main

import "fmt"

func hello() {
	fmt.Println("hello")
}

func world() {
	fmt.Println("world")
}

type Foo struct {
	Name string
}
`
	segs := Segmentize(code)
	if len(segs) == 0 {
		t.Fatal("expected segments, got none")
	}

	// Should find at least 2 functions and 1 type
	funcCount := 0
	typeCount := 0
	for _, s := range segs {
		if s.Kind == SegmentFunction {
			funcCount++
		}
		if s.Kind == SegmentClass {
			typeCount++
		}
	}
	if funcCount < 2 {
		t.Errorf("expected >= 2 function segments, got %d", funcCount)
	}
	if typeCount < 1 {
		t.Errorf("expected >= 1 type/class segment, got %d", typeCount)
	}
}

func TestSegmenter_Python(t *testing.T) {
	code := `import os

def greet(name):
    print(f"Hello {name}")

class User:
    def __init__(self, name):
        self.name = name

    def display(self):
        print(self.name)
`
	segs := Segmentize(code)
	if len(segs) == 0 {
		t.Fatal("expected segments, got none")
	}

	funcCount := 0
	classCount := 0
	for _, s := range segs {
		if s.Kind == SegmentFunction {
			funcCount++
		}
		if s.Kind == SegmentClass {
			classCount++
		}
	}
	if funcCount < 1 {
		t.Errorf("expected >= 1 function segment (def greet), got %d", funcCount)
	}
	if classCount < 1 {
		t.Errorf("expected >= 1 class segment, got %d", classCount)
	}
}

func TestSegmenter_JavaScript(t *testing.T) {
	code := `const express = require('express');

function handleRequest(req, res) {
    res.send('ok');
}

class Server {
    constructor(port) {
        this.port = port;
    }

    start() {
        console.log('starting');
    }
}
`
	segs := Segmentize(code)
	funcCount := 0
	classCount := 0
	for _, s := range segs {
		if s.Kind == SegmentFunction {
			funcCount++
		}
		if s.Kind == SegmentClass {
			classCount++
		}
	}
	if funcCount < 1 {
		t.Errorf("expected >= 1 function segment, got %d", funcCount)
	}
	if classCount < 1 {
		t.Errorf("expected >= 1 class segment, got %d", classCount)
	}
}

func TestSegmenter_Empty(t *testing.T) {
	segs := Segmentize("")
	if len(segs) != 0 {
		t.Errorf("expected 0 segments for empty input, got %d", len(segs))
	}
}

func TestSegmenter_NoFunctions(t *testing.T) {
	code := "x := 1\ny := 2\nz := x + y\n"
	segs := Segmentize(code)
	// Should still produce at least one SegmentLines block
	if len(segs) == 0 {
		t.Error("expected at least one segment for plain code")
	}
	if segs[0].Kind != SegmentLines {
		t.Errorf("expected SegmentLines, got %d", segs[0].Kind)
	}
}

func TestSegmenter_TokensPopulated(t *testing.T) {
	code := "func authenticateUser() {\n\treturn nil\n}\n"
	segs := Segmentize(code)
	if len(segs) == 0 {
		t.Fatal("expected segments")
	}
	// Tokens should be populated (7.6 pre-computation)
	if len(segs[0].Tokens) == 0 {
		t.Error("expected pre-computed tokens in segment")
	}
	if !containsToken(segs[0].Tokens, "authenticate") {
		t.Errorf("expected 'authenticate' in tokens, got %v", segs[0].Tokens)
	}
}

func TestSegmenter_Rust(t *testing.T) {
	code := `use std::io;

fn main() {
    println!("hello");
}

impl Server {
    fn new(port: u16) -> Self {
        Server { port }
    }
}
`
	segs := Segmentize(code)
	funcCount := 0
	for _, s := range segs {
		if s.Kind == SegmentFunction {
			funcCount++
		}
	}
	if funcCount < 1 {
		t.Errorf("expected >= 1 fn segment, got %d", funcCount)
	}
}

func TestSegmenter_Java(t *testing.T) {
	code := `import java.util.List;

public class Main {
    public static void main(String[] args) {
        System.out.println("hello");
    }

    private int calculate(int a, int b) {
        return a + b;
    }
}
`
	segs := Segmentize(code)
	classCount := 0
	for _, s := range segs {
		if s.Kind == SegmentClass {
			classCount++
		}
	}
	if classCount < 1 {
		t.Errorf("expected >= 1 class segment, got %d", classCount)
	}
}

func TestSegmenter_PreservesContent(t *testing.T) {
	code := "func foo() {\n\tx := 1\n\treturn x\n}\n"
	segs := Segmentize(code)
	if len(segs) == 0 {
		t.Fatal("expected segments")
	}
	// The function segment should contain the original content
	found := false
	for _, s := range segs {
		if strings.Contains(s.Content, "func foo()") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected segment containing 'func foo()'")
	}
}

func TestSegmenter_OneLineBraceBlock_DoesNotSwallowNextLine(t *testing.T) {
	code := "func a() {}\nnextLine()\n"
	segs := Segmentize(code)
	if len(segs) == 0 {
		t.Fatal("expected segments")
	}
	if segs[0].StartLine != 0 || segs[0].EndLine != 0 {
		t.Fatalf("expected one-line function segment [0,0], got [%d,%d]", segs[0].StartLine, segs[0].EndLine)
	}
}
