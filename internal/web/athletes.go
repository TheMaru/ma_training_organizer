package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// handleAthleteNew renders the empty create form. The roster's query travels
// through the form (in its action) so the created athlete lands back in the
// roster the trainer started from.
func (s *Server) handleAthleteNew(w http.ResponseWriter, r *http.Request) {
	query := rosterQueryFrom(r)
	s.renderAthleteForm(w, r, http.StatusOK, query, rosterPath, "athlete.new", store.Athlete{}, "")
}

// handleAthleteCreate validates the submitted form and inserts a new athlete,
// re-rendering the form with a 400 when required fields are missing.
func (s *Server) handleAthleteCreate(w http.ResponseWriter, r *http.Request) {
	query := rosterQueryFrom(r)
	a, errMsg := athleteFromForm(r)
	if errMsg != "" {
		s.renderAthleteForm(w, r, http.StatusBadRequest, query, rosterPath, "athlete.new", a, errMsg)
		return
	}
	if _, err := store.CreateAthlete(s.db, a); err != nil {
		serverError(w)
		return
	}
	redirect(w, r, query.path(rosterPath))
}

// handleAthleteEdit renders the edit form pre-filled from the stored athlete.
func (s *Server) handleAthleteEdit(w http.ResponseWriter, r *http.Request) {
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
	query := rosterQueryFrom(r)
	s.renderAthleteForm(w, r, http.StatusOK, query, athletePath(id), "athlete.edit", a, "")
}

// handleAthleteUpdate validates the form and overwrites the athlete's fields.
// A missing name re-renders the form (400); an unknown id is a 404.
func (s *Server) handleAthleteUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := athleteID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	query := rosterQueryFrom(r)
	a, errMsg := athleteFromForm(r)
	a.ID = id
	if errMsg != "" {
		s.renderAthleteForm(w, r, http.StatusBadRequest, query, athletePath(id), "athlete.edit", a, errMsg)
		return
	}
	err := store.UpdateAthlete(s.db, a)
	if errors.Is(err, store.ErrAthleteNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		serverError(w)
		return
	}
	redirect(w, r, query.path(rosterPath))
}

// handleAthleteDelete hard-deletes an athlete (cascading to their promotions).
func (s *Server) handleAthleteDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := athleteID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	err := store.DeleteAthlete(s.db, id)
	if errors.Is(err, store.ErrAthleteNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		serverError(w)
		return
	}
	redirect(w, r, rosterQueryFrom(r).path(rosterPath))
}

// athleteFromForm reads and trims the athlete form fields. It returns a non-empty
// message when a required field (first or last name) is missing; the returned
// Athlete is still populated so the form can be re-rendered with the user's input.
func athleteFromForm(r *http.Request) (store.Athlete, string) {
	a := store.Athlete{
		FirstName: strings.TrimSpace(r.PostFormValue("firstName")),
		LastName:  strings.TrimSpace(r.PostFormValue("lastName")),
		BirthDate: strings.TrimSpace(r.PostFormValue("birthDate")),
		JoinedOn:  strings.TrimSpace(r.PostFormValue("joinedOn")),
		Notes:     strings.TrimSpace(r.PostFormValue("notes")),
	}
	if a.FirstName == "" || a.LastName == "" {
		return a, translate(r, "athlete.nameRequired")
	}
	return a, ""
}

// athleteID parses the {id} route parameter, reporting false for a non-numeric
// value so the handler can answer 404 rather than 500.
func athleteID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

// The athlete URLs, spelled once. athletePath is both the detail page and — as a
// POST — the update target. They are all bare paths: a caller runs them through
// rosterQuery.path before they reach a template or a Location header.
const rosterPath = "/athletes"

func athletePath(id int64) string    { return fmt.Sprintf("%s/%d", rosterPath, id) }
func editPath(id int64) string       { return athletePath(id) + "/edit" }
func deletePath(id int64) string     { return athletePath(id) + "/delete" }
func promotionsPath(id int64) string { return athletePath(id) + "/promotions" }

// renderAthleteForm renders the create/edit form. query is the roster the form was
// reached from, and it is applied to both URLs here — to action, so submitting
// carries the state to the handler that redirects, and to "Abbrechen", so
// cancelling lands on the same roster. Callers pass the bare action path.
func (s *Server) renderAthleteForm(w http.ResponseWriter, r *http.Request, status int, query rosterQuery, action, headingKey string, a store.Athlete, errMsg string) {
	s.tmpl.render(w, r, status, "athlete_form.html", map[string]any{
		"Authenticated": true,
		"Action":        query.path(action),
		"Cancel":        query.path(rosterPath),
		"Heading":       translate(r, headingKey),
		"Athlete":       a,
		"Error":         errMsg,
	})
}
