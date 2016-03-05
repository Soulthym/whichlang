package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/unixpickle/weakai/idtrees"
	"github.com/unixpickle/whichlang"
)

func GenerateClassifier(freqs map[string][]whichlang.Frequencies) *whichlang.Classifier {
	allWords := map[string]bool{}
	entryCount := 0
	for _, samples := range freqs {
		for _, sample := range samples {
			for word := range sample {
				allWords[word] = true
			}
			entryCount++
		}
	}

	res := &whichlang.Classifier{}
	dataSet := &idtrees.DataSet{
		Entries: make([]idtrees.Entry, 0, entryCount),
		Fields:  make([]idtrees.Field, 0),
	}

	fmt.Println("Generating entries...")

	for lang, list := range freqs {
		for _, wordMap := range list {
			entry := &treeEntry{
				language:    lang,
				freqs:       wordMap,
				fieldValues: []idtrees.Value{},
			}
			dataSet.Entries = append(dataSet.Entries, entry)
		}
	}

	fmt.Println("Generating fields...")
	for word := range allWords {
		idtrees.CreateBisectingFloatFields(dataSet, func(e idtrees.Entry) float64 {
			return e.(*treeEntry).freqs[word]
		}, func(e idtrees.Entry, v idtrees.Value) {
			te := e.(*treeEntry)
			te.fieldValues = append(te.fieldValues, v)
		}, strings.Replace(word, "%", "%%", -1)+" > %f")
	}
	fmt.Println("Generating tree...")

	tree := idtrees.GenerateTree(dataSet)
	if tree == nil {
		fmt.Fprintln(os.Stderr, "Failed to generate tree.")
		os.Exit(1)
	}

	fmt.Println("Tree is:")
	fmt.Println(tree)

	res.TreeRoot = convertTree(tree)
	centerThresholds(res, freqs)
	return res
}

func convertTree(t *idtrees.TreeNode) *whichlang.ClassifierNode {
	if t.BranchField == nil {
		if t.LeafValue == nil {
			return &whichlang.ClassifierNode{
				Leaf:               true,
				LeafClassification: "Unknown",
			}
		} else {
			return &whichlang.ClassifierNode{
				Leaf:               true,
				LeafClassification: t.LeafValue.String(),
			}
		}
	}

	comps := strings.Split(t.BranchField.String(), " ")
	if len(comps) != 3 {
		panic("unknown branch field: " + t.BranchField.String())
	}
	val, _ := strconv.ParseFloat(comps[2], 64)
	res := &whichlang.ClassifierNode{
		Keyword:   comps[0],
		Threshold: val,
	}
	res.FalseBranch = convertTree(t.Branches[idtrees.BoolValue(false)])
	res.TrueBranch = convertTree(t.Branches[idtrees.BoolValue(true)])
	return res
}

func centerThresholds(c *whichlang.Classifier, f map[string][]whichlang.Frequencies) {
	vecs := []whichlang.Frequencies{}
	for _, list := range f {
		for _, wordMap := range list {
			vecs = append(vecs, wordMap)
		}
	}
	centerThresholdsForNode(vecs, c.TreeRoot)
}

func centerThresholdsForNode(vecs []whichlang.Frequencies, node *whichlang.ClassifierNode) {
	if node.Leaf {
		return
	}
	var lowerSide float64
	var upperSide float64
	for i, vec := range vecs {
		if vec[node.Keyword] <= node.Threshold {
			if i == 0 || vec[node.Keyword] > lowerSide {
				lowerSide = node.Threshold
			}
		} else {
			if i == 0 || vec[node.Keyword] < upperSide {
				upperSide = node.Threshold
			}
		}
	}
	node.Threshold = (lowerSide + upperSide) / 2

	t, f := splitOnNode(vecs, node)
	centerThresholdsForNode(t, node.TrueBranch)
	centerThresholdsForNode(f, node.FalseBranch)
}

func splitOnNode(vecs []whichlang.Frequencies,
	node *whichlang.ClassifierNode) (t, f []whichlang.Frequencies) {
	t = make([]whichlang.Frequencies, 0, len(vecs))
	f = make([]whichlang.Frequencies, 0, len(vecs))
	for _, v := range vecs {
		if v[node.Keyword] > node.Threshold {
			t = append(t, v)
		} else {
			f = append(f, v)
		}
	}
	return
}

type treeEntry struct {
	language    string
	freqs       whichlang.Frequencies
	fieldValues []idtrees.Value
}

func (t *treeEntry) FieldValues() []idtrees.Value {
	return t.fieldValues
}

func (t *treeEntry) Class() idtrees.Value {
	return idtrees.StringValue(t.language)
}