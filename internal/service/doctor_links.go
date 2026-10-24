package service

import (
	"encoding/json"
	"fmt"
	"os"
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
	finished := map[doctorLinkNode]bool{}
	active := map[doctorLinkNode]int{}
	var path []doctorLinkNode
	var visit func(doctorLinkNode)
	visit = func(node doctorLinkNode) {
		if index, exists := active[node]; exists {
			cycle := path[index:]
			anchor := cycle[0]
			if onlyBoard != "" {
				found := false
				for _, member := range cycle {
					if member.board == onlyBoard {
						anchor, found = member, true
						break
					}
				}
				if !found {
					return
				}
			}
			refs := make([]string, 0, len(cycle)+1)
			for _, member := range cycle {
				refs = append(refs, member.board+":"+member.id)
			}
			refs = append(refs, node.board+":"+node.id)
			report.Issues = append(report.Issues, Issue{
				Severity: SeverityWarning, Code: CodeBlockedByCycle, Board: anchor.board,
				CardID: anchor.id, Message: "Blocker cycle: " + strings.Join(refs, " -> "),
			})
			return
		}
		if finished[node] {
			return
		}
		active[node] = len(path)
		path = append(path, node)
		card := graph.boards[node.board].cards[node.id]
		seen := map[doctorLinkNode]bool{}
		for _, ref := range card.BlockedBy {
			result := graph.Resolve(node.board, ref)
			if result.State == LinkFound {
				target := doctorLinkNode{board: result.BoardName, id: result.Card.ID}
				if !seen[target] {
					seen[target] = true
					visit(target)
				}
			}
		}
		path = path[:len(path)-1]
		delete(active, node)
		finished[node] = true
	}
	for _, name := range graph.names {
		cards, err := graph.Cards(name)
		if err != nil {
			continue
		}
		cards = append([]*model.Card(nil), cards...)
		sort.Slice(cards, func(i, j int) bool { return cards[i].ID < cards[j].ID })
		for _, card := range cards {
			visit(doctorLinkNode{board: name, id: card.ID})
		}
	}
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
