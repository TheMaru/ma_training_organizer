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

// handleAthletesList renders the shared roster ordered by last name. The list is
// sortable: ?dir=desc flips the direction, and the column header links to the
// opposite direction so a click toggles it.
func (s *Server) handleAthletesList(w http.ResponseWriter, r *http.Request) {
	descending := r.URL.Query().Get("dir") == "desc"

	athletes, err := store.ListAthletes(s.db, descending)
	if err != nil {
		serverError(w)
		return
	}

	nextDir := "desc"
	if descending {
		nextDir = "asc"
	}
	s.tmpl.render(w, http.StatusOK, "athletes.html", map[string]any{
		"Authenticated": true,
		"Athletes":      athletes,
		"NextDir":       nextDir,
	})
}

// handleAthleteNew renders the empty create form.
func (s *Server) handleAthleteNew(w http.ResponseWriter, _ *http.Request) {
	s.renderAthleteForm(w, http.StatusOK, "/athletes", "Neuer Athlet", store.Athlete{}, "")
}

// handleAthleteCreate validates the submitted form and inserts a new athlete,
// re-rendering the form with a 400 when required fields are missing.
func (s *Server) handleAthleteCreate(w http.ResponseWriter, r *http.Request) {
	a, errMsg := athleteFromForm(r)
	if errMsg != "" {
		s.renderAthleteForm(w, http.StatusBadRequest, "/athletes", "Neuer Athlet", a, errMsg)
		return
	}
	if _, err := store.CreateAthlete(s.db, a); err != nil {
		serverError(w)
		return
	}
	redirect(w, r, "/athletes")
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
	s.renderAthleteForm(w, http.StatusOK, editAction(id), "Athlet bearbeiten", a, "")
}

// handleAthleteUpdate validates the form and overwrites the athlete's fields.
// A missing name re-renders the form (400); an unknown id is a 404.
func (s *Server) handleAthleteUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := athleteID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	a, errMsg := athleteFromForm(r)
	a.ID = id
	if errMsg != "" {
		s.renderAthleteForm(w, http.StatusBadRequest, editAction(id), "Athlet bearbeiten", a, errMsg)
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
	redirect(w, r, "/athletes")
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
	redirect(w, r, "/athletes")
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
		return a, "Vor- und Nachname sind erforderlich."
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

// editAction is the form/post target for editing a given athlete.
func editAction(id int64) string {
	return fmt.Sprintf("/athletes/%d", id)
}

func (s *Server) renderAthleteForm(w http.ResponseWriter, status int, action, heading string, a store.Athlete, errMsg string) {
	s.tmpl.render(w, status, "athlete_form.html", map[string]any{
		"Authenticated": true,
		"Action":        action,
		"Heading":       heading,
		"Athlete":       a,
		"Error":         errMsg,
	})
}
