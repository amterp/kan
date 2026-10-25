package api

import (
	"log"
	"net/http"
	"strings"

	"github.com/amterp/kan/internal/model"
	"github.com/amterp/kan/internal/registry"
	"github.com/amterp/kan/internal/service"
)

type CardLinkResponse struct {
	Ref           string            `json:"ref"`
	Result        service.LinkState `json:"result"`
	BoardID       string            `json:"board_id,omitempty"`
	Board         string            `json:"board,omitempty"`
	ProjectPath   string            `json:"project_path,omitempty"`
	ProjectName   string            `json:"project_name,omitempty"`
	Category      string            `json:"category,omitempty"`
	Reason        string            `json:"reason,omitempty"`
	MinKanVersion string            `json:"min_kan_version,omitempty"`
	LinkedName    string            `json:"linked_name,omitempty"`
	LinkedRemote  string            `json:"linked_remote,omitempty"`
	Card          *CardResponse     `json:"card,omitempty"`
}

type BoardLinksResponse struct {
	Resolved map[string]CardLinkResponse   `json:"resolved"`
	Blocks   map[string][]CardLinkResponse `json:"blocks"`
}

type LinkedCardResponse struct {
	CardLinkResponse
	ReadOnly    bool                `json:"read_only"`
	BoardConfig *model.BoardConfig  `json:"board_config,omitempty"`
	Links       *BoardLinksResponse `json:"links,omitempty"`
}

type linkSnapshot struct {
	graph *service.LinkGraph
	ctx   *ProjectContext
	cfg   *model.GlobalConfig
}

func (h *Handler) loadLinks(ctx *ProjectContext) (*linkSnapshot, error) {
	cfg := &model.GlobalConfig{}
	if h.globalStore != nil {
		var err error
		loaded, err := h.globalStore.Load()
		if err != nil {
			log.Printf("Warning: can't read the project registry in the global config: %v. "+
				"The board still loads, but links to boards in other projects show as not on this machine "+
				"until the global config can be read again; check that file for a syntax error.", err)
		} else {
			cfg = loaded
		}
	}
	graph, err := service.LoadLinkGraph(ctx.BoardStore, ctx.CardStore)
	if err != nil {
		return nil, err
	}
	graph.SetElsewhere(service.NewElsewhere(ctx.Paths, ctx.BoardStore, ctx.ProjectStore, cfg))
	graph.LoadOtherProjects()
	return &linkSnapshot{graph: graph, ctx: ctx, cfg: cfg}, nil
}

func (s *linkSnapshot) response(found service.LinkLookup) CardLinkResponse {
	result := CardLinkResponse{
		Ref: found.Ref, Result: found.State, BoardID: found.BoardID, Board: found.BoardName,
		ProjectPath: found.Project, Reason: found.Reason, MinKanVersion: found.MinKanVersion,
		LinkedName: found.LinkedName, LinkedRemote: found.LinkedRemote,
	}
	board, err := s.graph.LookupBoard(found)
	if found.Project == "" && (err != nil || board.ID != found.BoardID) {
		result.Board = ""
	}
	if result.ProjectPath == "" && err == nil && board.ID == found.BoardID {
		result.ProjectPath = s.ctx.ProjectRoot
	}
	if result.ProjectPath != "" {
		dataLocation := s.ctx.Paths.DataLocation()
		if result.ProjectPath != s.ctx.ProjectRoot {
			dataLocation = ""
			if s.cfg != nil {
				if repo := s.cfg.GetRepoConfig(result.ProjectPath); repo != nil {
					dataLocation = repo.DataLocation
				}
			}
		}
		result.ProjectName = registry.ProjectName(result.ProjectPath, dataLocation)
	}
	if found.State == service.LinkFound {
		result.Ref = found.BoardID + ":" + found.Card.ID
		card := toCardResponseWithWanted(found.Card, board)
		result.Card = &card
		if found.Column != nil {
			result.Category = found.Column.Category
		}
	}
	return result
}

func (s *linkSnapshot) boardLinks(boardName string, cards []*model.Card, includeLocalReverse bool) *BoardLinksResponse {
	result := &BoardLinksResponse{Resolved: map[string]CardLinkResponse{}, Blocks: map[string][]CardLinkResponse{}}
	for _, card := range cards {
		s.addCardLinks(result, s.graph.Resolve(boardName, card.ID), includeLocalReverse)
	}
	return result
}

func (h *Handler) GetLinkedCard(w http.ResponseWriter, r *http.Request) {
	ref := r.PathValue("ref")
	boardID, cardID, qualified := strings.Cut(ref, ":")
	if !qualified || boardID == "" || cardID == "" {
		BadRequest(w, "Expected boardID:cardID")
		return
	}
	ctx := h.ctx()
	snapshot, err := h.loadLinks(ctx)
	if err != nil {
		Error(w, err)
		return
	}
	found := snapshot.graph.Resolve("", ref)
	result := LinkedCardResponse{CardLinkResponse: snapshot.response(found), ReadOnly: true}
	if found.State == service.LinkFound {
		result.BoardConfig, err = snapshot.graph.LookupBoard(found)
		if err != nil {
			Error(w, err)
			return
		}
		result.Links = snapshot.linksForLookup(found)
	}
	JSON(w, http.StatusOK, result)
}

func (s *linkSnapshot) linksForLookup(found service.LinkLookup) *BoardLinksResponse {
	result := &BoardLinksResponse{Resolved: map[string]CardLinkResponse{}, Blocks: map[string][]CardLinkResponse{}}
	s.addCardLinks(result, found, true)
	return result
}

func (s *linkSnapshot) addCardLinks(result *BoardLinksResponse, found service.LinkLookup, includeLocalReverse bool) {
	for _, refs := range [][]string{found.Card.BlockedBy, found.Card.Related} {
		for _, ref := range refs {
			if _, exists := result.Resolved[ref]; !exists {
				result.Resolved[ref] = s.response(s.graph.ResolveFrom(found, ref))
			}
		}
	}
	for _, incoming := range s.graph.BlocksFor(found) {
		if !includeLocalReverse && incoming.Project == found.Project && incoming.BoardID == found.BoardID {
			continue
		}
		result.Blocks[found.Card.ID] = append(result.Blocks[found.Card.ID], s.response(incoming))
	}
}
