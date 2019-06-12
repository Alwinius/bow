package main

import (
	"fmt"
	"github.com/alwinius/keel/provider/helm"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
)

// Example of how to:
// - Clone a repository into memory
// - Get the HEAD reference
// - Using the HEAD reference, obtain the commit this reference is pointing to
// - Using the commit, obtain its history and print it
func main() {
	path := "./"

	// Clones the given repository, creating the remote, the local branches
	// and fetching the objects, everything in memory:
	fmt.Println("git clone")

	var r *git.Repository
	var err error
	if r, err = git.PlainOpen(path); err != nil {
		r, err = git.PlainClone("./", false, &git.CloneOptions{
			Auth: &http.BasicAuth{},
			URL:  "https://iteragit.iteratec.de/bachelors-thesis-aeb/petclinic-deployment.git",
		})
	}

	// Gets the HEAD history from HEAD, just like this command:
	fmt.Println("git log")

	// ... retrieves the branch pointed by HEAD
	ref, err := r.Head()

	// ... retrieving the commit object
	commit, err := r.CommitObject(ref.Hash())
	fmt.Println(commit)

	// List the tree from HEAD
	fmt.Println("git ls-tree -r HEAD")

	//// ... retrieve the tree from the commit
	//tree, err := commit.Tree()
	//CheckIfError(err)
	//
	//// ... get the files iterator and print the file
	//tree.Files().ForEach(func(f *object.File) error {
	//	fmt.Printf("100644 blob %s    %s\n", f.Hash, f.Name)
	//	return nil
	//})

	err = helm.ProcessTemplate("./helm/petclinic/")
	if err != nil {
		fmt.Println(err)
	}
}
