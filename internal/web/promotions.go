package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// handleAthleteDetail renders one athlete with their derived current rank, full
// promotion history and a form to record a new promotion.
func (s *Server) handleAthleteDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := athleteID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a, err := store.AthleteByID(s.db, id)
	if errors.Is(err, store.ErrAthleteNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		serverError(w)
		return
	}
	s.renderAthleteDetail(w, http.StatusOK, rosterViewFrom(r), a, "")
}

// handleAthletePromote records a promotion for the athlete, then redirects back
// to their detail view where the updated current rank shows. A missing rank or
// date re-renders the detail with a 400; an unknown athlete is a 404.
func (s *Server) handleAthletePromote(w http.ResponseWriter, r *http.Request) {
	id, ok := athleteID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a, err := store.AthleteByID(s.db, id)
	if errors.Is(err, store.ErrAthleteNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		serverError(w)
		return
	}

	view := rosterViewFrom(r)
	rankID, _ := strconv.ParseInt(r.PostFormValue("rankId"), 10, 64)
	date := strings.TrimSpace(r.PostFormValue("promotedOn"))
	// A malformed date must be rejected here: the DATE column would otherwise take
	// the garbage string and every later read would fail its NullTime scan (500).
	// <input type="date"> guards the UI; this guards a hand-crafted POST.
	if _, err := time.Parse("2006-01-02", date); rankID == 0 || err != nil {
		s.renderAthleteDetail(w, http.StatusBadRequest, view, a, "Bitte Rang und Datum wählen.")
		return
	}

	_, err = store.CreatePromotion(s.db, store.Promotion{AthleteID: id, RankID: rankID, PromotedOn: date})
	if err != nil {
		serverError(w)
		return
	}
	redirect(w, r, view.path(athletePath(id)))
}

// renderAthleteDetail loads the athlete's promotion history and the grading
// systems for the record form, derives the current rank, and renders the detail
// page. errMsg is shown when re-rendering after a rejected record attempt. view
// is the roster this page was opened from: every link and form on the page keeps
// carrying it, so the way back stays the sorted roster.
func (s *Server) renderAthleteDetail(w http.ResponseWriter, status int, view rosterView, a store.Athlete, errMsg string) {
	promotions, err := store.ListPromotions(s.db, a.ID)
	if err != nil {
		serverError(w)
		return
	}
	systems, err := store.ListGradingSystems(s.db)
	if err != nil {
		serverError(w)
		return
	}
	current, hasCurrent := store.CurrentRank(promotions)

	s.tmpl.render(w, status, "athlete_detail.html", map[string]any{
		"Authenticated": true,
		"Athlete":       a,
		"Promotions":    promotions,
		"Current":       current,
		"HasCurrent":    hasCurrent,
		"Systems":       systems,
		"Error":         errMsg,
		"EditHref":      view.path(editPath(a.ID)),
		"PromoteAction": view.path(promotionsPath(a.ID)),
		"Back":          view.path(rosterPath),
	})
}
