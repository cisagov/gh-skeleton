package main

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
)

// GraphQLClient abstracts the GitHub GraphQL API.
type GraphQLClient interface {
	Do(query string, variables map[string]interface{}, response interface{}) error
}

type searchResult struct {
	Search struct {
		RepositoryCount int
		Edges           []struct {
			Node struct {
				Name        string
				Description string
			}
		}
	}
}

func listSkeletons(srcOrg string, client GraphQLClient) error {
	query := `
query($query: String!) {
  search(query: $query, type: REPOSITORY, first: 100) {
    repositoryCount
    edges {
      node {
        ... on Repository {
          name
          description
        }
      }
    }
  }
}`

	variables := map[string]interface{}{
		"query": "org:" + srcOrg + " topic:skeleton archived:false",
	}

	var result searchResult
	if err := client.Do(query, variables, &result); err != nil {
		return fmt.Errorf("graphql query failed: %w", err)
	}

	fmt.Printf("Available skeletons in %s:\n\n", srcOrg)

	type repo struct {
		name        string
		description string
	}
	repos := make([]repo, 0, len(result.Search.Edges))
	for _, edge := range result.Search.Edges {
		repos = append(repos, repo{
			name:        edge.Node.Name,
			description: edge.Node.Description,
		})
	}
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].name < repos[j].name
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	for _, r := range repos {
		fmt.Fprintf(w, "%-25s\t%s\n", r.name, r.description)
	}

	return w.Flush()
}
