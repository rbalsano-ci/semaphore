package projects

import (
	log "github.com/Sirupsen/logrus"
	"github.com/ansible-semaphore/semaphore/api/helpers"
	"github.com/ansible-semaphore/semaphore/db"
	"github.com/gorilla/context"
	"net/http"
	"strconv"
	"os"
	"net/url"
)

// TemplatesMiddleware ensures a template exists and loads it to the context
func TemplatesMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		project := context.Get(r, "project").(db.Project)
		templateID, err := helpers.GetIntParam("template_id", w, r)
		if err != nil {
			return
		}

		template, err := helpers.Store(r).GetTemplate(project.ID, templateID)

		if err != nil {
			helpers.WriteError(w, err)
			return
		}

		context.Set(r, "template", template)
		next.ServeHTTP(w, r)
	})
}

// GetTemplate returns single template by ID
func GetTemplate(w http.ResponseWriter, r *http.Request) {
	template := context.Get(r, "template").(db.Template)
	helpers.WriteJSON(w, http.StatusOK, template)
}

func GetTemplateRefs(w http.ResponseWriter, r *http.Request) {
	tpl := context.Get(r, "template").(db.Template)
	refs, err := helpers.Store(r).GetTemplateRefs(tpl.ProjectID, tpl.ID)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, refs)
}

// GetTemplates returns all templates for a project in a sort order
func GetTemplates(w http.ResponseWriter, r *http.Request) {
	project := context.Get(r, "project").(db.Project)

	templates, err := helpers.Store(r).GetTemplates(project.ID, db.TemplateFilter{}, helpers.QueryParams(r.URL))

	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, templates)
}

// GetInventoryContents returns text of inventory
func GetInventoryContents(w http.ResponseWriter, r *http.Request) {
	template := context.Get(r, "template").(db.Template)
	inventory, err := helpers.Store(r).GetInventory(template.ProjectID, template.InventoryID)

	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	if (inventory.Type == "file") {
		repository, err := helpers.Store(r).GetRepository(template.ProjectID, template.RepositoryID)
		if err != nil {
		        helpers.WriteError(w, err)
			return
		}

		fileUrl, _ := url.Parse(repository.GitURL + inventory.Inventory)
		contents, _ := os.ReadFile(fileUrl.Path)
		helpers.WriteJSON(w, http.StatusOK, string(contents))
	} else if (inventory.Type == "static") {
		contents := inventory.Inventory
		helpers.WriteJSON(w, http.StatusOK, contents)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

// PutInventoryContents creates or updates an inventory
func PutInventoryContents(w http.ResponseWriter, r *http.Request) {
	template := context.Get(r, "template").(db.Template)
	inventory, err := helpers.Store(r).GetInventory(template.ProjectID, template.InventoryID)

	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	var host db.Host
	if !helpers.Bind(w, r, &host) {
		return
	}

	contents := "[cortex_connect]\n"+host.Name+" ansible_host=" + host.Ip + "\n\n[all:vars]\nansible_python_interpreter=/usr/bin/python3\n"

	if (inventory.Type == "file") {
		 repository, err := helpers.Store(r).GetRepository(template.ProjectID, template.RepositoryID)
		 if err != nil {
			helpers.WriteError(w, err)
			return
		}

		fileUrl, _ := url.Parse(repository.GitURL + inventory.Inventory)
		err = os.WriteFile(fileUrl.Path, []byte(contents), 0644)
		if err != nil {
			helpers.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	} else if (inventory.Type == "static") {
		inventory.Inventory = contents
		err := helpers.Store(r).UpdateInventory(inventory)
		if err != nil {
			helpers.WriteError(w, err)
			return
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

// DeleteInventoryContents delete an inventory
func DeleteInventoryContents(w http.ResponseWriter, r *http.Request) {
	template := context.Get(r, "template").(db.Template)
	inventory, err := helpers.Store(r).GetInventory(template.ProjectID, template.InventoryID)

	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	if (inventory.Type == "file") {
		repository, err := helpers.Store(r).GetRepository(template.ProjectID, template.RepositoryID)
		if err != nil {
			helpers.WriteError(w, err)
			return
		}

		fileUrl, _ := url.Parse(repository.GitURL + inventory.Inventory)
		err = os.Remove(fileUrl.Path)
		if err != nil {
			helpers.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	} else if (inventory.Type == "static") {
		inventory.Inventory = ""
		err := helpers.Store(r).UpdateInventory(inventory)
		if err != nil {
			helpers.WriteError(w, err)
			return
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

// AddTemplate adds a template to the database
func AddTemplate(w http.ResponseWriter, r *http.Request) {
	project := context.Get(r, "project").(db.Project)

	var template db.Template
	if !helpers.Bind(w, r, &template) {
		return
	}

	template.ProjectID = project.ID
	newTemplate, err := helpers.Store(r).CreateTemplate(template)

	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	user := context.Get(r, "user").(*db.User)
	objType := db.EventTemplate
	desc := "Template ID " + strconv.Itoa(newTemplate.ID) + " created"

	_, err = helpers.Store(r).CreateEvent(db.Event{
		UserID:      &user.ID,
		ProjectID:   &project.ID,
		ObjectType:  &objType,
		ObjectID:    &newTemplate.ID,
		Description: &desc,
	})

	if err != nil {
		log.Error(err)
	}

	helpers.WriteJSON(w, http.StatusCreated, newTemplate)
}

// UpdateTemplate writes a template to an existing key in the database
func UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	oldTemplate := context.Get(r, "template").(db.Template)

	var template db.Template
	if !helpers.Bind(w, r, &template) {
		return
	}

	// project ID and template ID in the body and the path must be the same

	if template.ID != oldTemplate.ID {
		helpers.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "template id in URL and in body must be the same",
		})
		return
	}

	if template.ProjectID != oldTemplate.ProjectID {
		helpers.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "You can not move template to other project",
		})
		return
	}

	if template.Arguments != nil && *template.Arguments == "" {
		template.Arguments = nil
	}

	err := helpers.Store(r).UpdateTemplate(template)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	user := context.Get(r, "user").(*db.User)

	desc := "Template ID " + strconv.Itoa(template.ID) + " updated"
	objType := db.EventTemplate

	_, err = helpers.Store(r).CreateEvent(db.Event{
		UserID:      &user.ID,
		ProjectID:   &template.ProjectID,
		Description: &desc,
		ObjectID:    &template.ID,
		ObjectType:  &objType,
	})

	if err != nil {
		log.Error(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

// RemoveTemplate deletes a template from the database
func RemoveTemplate(w http.ResponseWriter, r *http.Request) {
	tpl := context.Get(r, "template").(db.Template)

	err := helpers.Store(r).DeleteTemplate(tpl.ProjectID, tpl.ID)
	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	user := context.Get(r, "user").(*db.User)
	desc := "Template ID " + strconv.Itoa(tpl.ID) + " deleted"
	_, err = helpers.Store(r).CreateEvent(db.Event{
		UserID:      &user.ID,
		ProjectID:   &tpl.ProjectID,
		Description: &desc,
	})

	if err != nil {
		log.Error(err)
	}

	w.WriteHeader(http.StatusNoContent)
}
