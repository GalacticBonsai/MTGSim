package main

import (
	"testing"
)

func TestAddWin(t *testing.T) {
	results = []winner{}
	AddWin("Deck A")
	if len(results) != 1 || results[0].Wins != 1 {
		t.Errorf("Expected Deck A to have 1 win")
	}
	AddWin("Deck A")
	if results[0].Wins != 2 {
		t.Errorf("Expected Deck A to have 2 wins")
	}
}

func TestAddLoss(t *testing.T) {
	results = []winner{}
	AddLoss("Deck B")
	if len(results) != 1 || results[0].Losses != 1 {
		t.Errorf("Expected Deck B to have 1 loss")
	}
	AddLoss("Deck B")
	if results[0].Losses != 2 {
		t.Errorf("Expected Deck B to have 2 losses")
	}
}

func TestSortWinners(t *testing.T) {
	results = []winner{
		{Name: "Deck A", Wins: 3, Losses: 1},
		{Name: "Deck B", Wins: 2, Losses: 1},
		{Name: "Deck C", Wins: 1, Losses: 1},
	}
	SortWinners()
	if results[0].Name != "Deck A" {
		t.Errorf("Expected Deck A to be first")
	}
	if results[1].Name != "Deck B" {
		t.Errorf("Expected Deck B to be second")
	}
	if results[2].Name != "Deck C" {
		t.Errorf("Expected Deck C to be third")
	}
}
