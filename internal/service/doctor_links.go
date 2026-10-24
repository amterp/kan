package service

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/store"
	"github.com/amterp/kan/internal/version"
)

func (s *DoctorService) checkCardLinks(report *DiagnosticReport, boardName string, cfg *model.BoardConfig, rawCards map[string]*model.Card, graph *LinkGraph) {
	fileIDs := make([]string, 0, len(rawCards))
	for id := range rawCards {
		fileIDs = append(fileIDs, id)
	}
	sort.Strings(fileIDs)
	parents := graph.boards[boardName].parents
	for _, fileID := range fileIDs {
		card := rawCards[fileID]
		// Older fields with these names are custom data until card/4 migration.
		if card.Version != version.CurrentCardVersion || len(card.BlockedBy)+len(card.Related) == 0 {
			continue
		}
		if !hasLinkCategory(cfg.GetColumn(card.Column)) {
			report.Issues = append(report.Issues, Issue{
				Severity: SeverityWarning, Code: CodeUncategorizedLink, Board: boardName,
				CardID: fileID, Column: card.Column,
				Message: fmt.Sprintf("Card with links is in column %q, which has no usable category", card.Column),
			})
		}
		ancestors := map[string]bool{}
		for parent := parents.resolve(card.Parent); parent != nil && !ancestors[parent.ID]; parent = parents.resolve(parent.Parent) {
			ancestors[parent.ID] = true
		}
		for _, field := range []struct {
			name string
			refs []string
		}{{"blocked_by", card.BlockedBy}, {"related", card.Related}} {
			seen := map[string]bool{}
			for _, ref := range field.refs {
				if seen[ref] {
					continue
				}
				seen[ref] = true
				result := graph.Resolve(boardName, ref)
				issue := Issue{Severity: SeverityWarning, Board: boardName, CardID: fileID,
					FixContext: map[string]string{"field": field.name, "ref": ref}}
				if result.State != LinkFound {
					issue.Code = CodeUnresolvedLink
					issue.Message = fmt.Sprintf("%s %q: %s: %s", field.name, ref, result.State, result.Reason)
					if result.State == LinkAbsent {
						issue.Code, issue.Fixable = CodeAbsentLink, true
						issue.FixAction = fmt.Sprintf("Remove %q from %s after confirming absence again", ref, field.name)
					}
					report.Issues = append(report.Issues, issue)
					continue
				}
				if !hasLinkCategory(result.Column) {
					issue.Code = CodeUncategorizedLink
					issue.Message = fmt.Sprintf("%s %q is in board %q column %q, which has no usable category", field.name, ref, result.BoardName, result.Card.Column)
					report.Issues = append(report.Issues, issue)
				}
				if field.name == "blocked_by" && result.BoardName == boardName && ancestors[result.Card.ID] {
					issue.Code = CodeBlockedByAncestor
					issue.Message = fmt.Sprintf("blocked_by %q names this card's ancestor %s", ref, result.Card.ID)
					report.Issues = append(report.Issues, issue)
				}
			}
		}
	}
}

func hasLinkCategory(column *model.Column) bool {
	return column != nil && column.Category != "" && model.IsValidCategory(column.Category)
}

type doctorLinkNode struct {
	board string
	id    string
}

func (s *DoctorService) checkBlockerCycles(report *DiagnosticReport, graph *LinkGraph, onlyBoard string) {
	var nodes []doctorLinkNode
	edges := map[doctorLinkNode][]doctorLinkNode{}
	incoming := map[doctorLinkNode][]doctorLinkNode{}
	for _, name := range graph.names {
		cards, err := graph.Cards(name)
		if err != nil {
			continue
		}
		cards = append([]*model.Card(nil), cards...)
		sort.Slice(cards, func(i, j int) bool { return cards[i].ID < cards[j].ID })
		for _, card := range cards {
			node := doctorLinkNode{board: name, id: card.ID}
			nodes = append(nodes, node)
			seen := map[doctorLinkNode]bool{}
			for _, ref := range card.BlockedBy {
				result := graph.Resolve(name, ref)
				if result.State != LinkFound {
					continue
				}
				target := doctorLinkNode{board: result.BoardName, id: result.Card.ID}
				if !seen[target] {
					seen[target] = true
					edges[node] = append(edges[node], target)
					incoming[target] = append(incoming[target], node)
				}
			}
		}
	}
	// Find strongly connected components before filtering: a DFS back edge can
	// omit other boards that participate in the same cyclic component.
	visited := map[doctorLinkNode]bool{}
	var order []doctorLinkNode
	var finish func(doctorLinkNode)
	finish = func(node doctorLinkNode) {
		if visited[node] {
			return
		}
		visited[node] = true
		for _, target := range edges[node] {
			finish(target)
		}
		order = append(order, node)
	}
	for _, node := range nodes {
		finish(node)
	}
	visited = map[doctorLinkNode]bool{}
	for i := len(order) - 1; i >= 0; i-- {
		if visited[order[i]] {
			continue
		}
		var component []doctorLinkNode
		members := map[doctorLinkNode]bool{}
		var collect func(doctorLinkNode)
		collect = func(node doctorLinkNode) {
			if visited[node] {
				return
			}
			visited[node], members[node] = true, true
			component = append(component, node)
			for _, source := range incoming[node] {
				collect(source)
			}
		}
		collect(order[i])
		if len(component) == 1 && !slices.Contains(edges[component[0]], component[0]) {
			continue
		}
		sort.Slice(component, func(i, j int) bool {
			if component[i].board != component[j].board {
				return component[i].board < component[j].board
			}
			return component[i].id < component[j].id
		})
		for _, anchor := range component {
			if onlyBoard != "" && anchor.board != onlyBoard {
				continue
			}
			cycle := blockerCycleThrough(anchor, members, edges)
			refs := make([]string, 0, len(cycle))
			for _, node := range cycle {
				refs = append(refs, node.board+":"+node.id)
			}
			report.Issues = append(report.Issues, Issue{
				Severity: SeverityWarning, Code: CodeBlockedByCycle, Board: anchor.board,
				CardID: anchor.id, Message: "Blocker cycle: " + strings.Join(refs, " -> "),
			})
			break
		}
	}
}

func blockerCycleThrough(anchor doctorLinkNode, members map[doctorLinkNode]bool, edges map[doctorLinkNode][]doctorLinkNode) []doctorLinkNode {
	path := []doctorLinkNode{anchor}
	visited := map[doctorLinkNode]bool{}
	var find func(doctorLinkNode) bool
	find = func(node doctorLinkNode) bool {
		visited[node] = true
		for _, target := range edges[node] {
			if !members[target] {
				continue
			}
			if target == anchor {
				path = append(path, anchor)
				return true
			}
			if !visited[target] {
				path = append(path, target)
				if find(target) {
					return true
				}
				path = path[:len(path)-1]
			}
		}
		return false
	}
	find(anchor)
	return path
}

func (s *DoctorService) fixAbsentLink(issue Issue) error {
	field, ref := issue.FixContext["field"], issue.FixContext["ref"]
	if field != "blocked_by" && field != "related" {
		return fmt.Errorf("invalid link field %q", field)
	}
	path := s.paths.CardPath(issue.Board, issue.CardID)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var cardVersion int
	if err := json.Unmarshal(raw["_v"], &cardVersion); err != nil || cardVersion != version.CurrentCardVersion {
		return fmt.Errorf("source card must use card/%d before links can be repaired", version.CurrentCardVersion)
	}
	if _, exists := raw[field]; !exists {
		return nil
	}
	var refs []string
	if err := json.Unmarshal(raw[field], &refs); err != nil {
		return err
	}
	kept := make([]string, 0, len(refs))
	for _, existing := range refs {
		if existing != ref {
			kept = append(kept, existing)
		}
	}
	if len(kept) == len(refs) {
		return nil
	}
	// Diagnosis may use an overlay; repair must recheck the files on disk.
	graph, err := LoadLinkGraph(store.NewBoardStore(s.paths), store.NewCardStore(s.paths))
	if err != nil {
		return err
	}
	if result := graph.Resolve(issue.Board, ref); result.State != LinkAbsent {
		return fmt.Errorf("%s %q is no longer confirmed absent: %s: %s", field, ref, result.State, result.Reason)
	}
	if len(kept) == 0 {
		delete(raw, field)
	} else {
		raw[field], err = json.Marshal(kept)
		if err != nil {
			return err
		}
	}
	output, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, output, 0644)
}
