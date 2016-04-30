package main

import (
	"testing"
)

func TestStripChars(t *testing.T) {
	testSlice := []string{
		"123_asdfa,",
		"asdf4x2X---",
		"  807C2-",
	}

	goodSlice := []string{
		"_asdf",
		"df4x",
		"807",
	}

	stripChars(&testSlice, "123as-XC, ")

	for i, testValue := range testSlice {
		if testValue != goodSlice[i] {
			t.Errorf("|%s| wasn't equal to known good |%s|\n", testValue, goodSlice[i])
		}
	}
}

func TestRemoveDupes(t *testing.T) {

	testSlice := []string{"a", "bird", "in", "the", "the", "bush", "a"}

	removeDuplicates(&testSlice)

	knownWords := make(map[string]bool)

	for _, word := range testSlice {
		if _, exists := knownWords[word]; exists {
			t.Errorf("Duplicate |%s| should have been removed!\n", word)
		} else {
			knownWords[word] = true
		}
	}
}

func TestNormalizeSlugs(t *testing.T) {
	inputs := []string{
		"Hello, world!",
		"😀  Ayy LMAO",
		"Nobody Expects the Spanish Inquisition!!!",
	}

	outputs := []string{
		"hello-world",
		"ayy-lmao",
		"nobody-expects-the-spanish-inquisition",
	}

	for i, input := range inputs {
		norm := NormalizeSlug(input, "post")
		if norm != outputs[i] {
			t.Errorf("|%s| was supposed to normalize to |%s|\n", norm, outputs[i])
		}
	}
}
