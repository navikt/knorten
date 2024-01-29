package api

import (
	"context"
	"encoding/gob"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/k8s"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type diffValue struct {
	Old       string
	New       string
	Encrypted string
}

type teamInfo struct {
	gensql.Team
	Namespace string
	Apps      []gensql.ChartType
	Events    []gensql.Event
}

func (c *client) setupAdminRoutes() {
	c.router.GET("/admin", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		flashes := session.Flashes()
		err := session.Save()
		if err != nil {
			c.log.WithError(err).Error("problem saving session")
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err})
			return
		}

		teams, err := c.repo.TeamsGet(ctx)
		if err != nil {
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err})
				return
			}

			ctx.Redirect(http.StatusSeeOther, "/admin")
			return
		}

		teamApps := map[string]teamInfo{}
		for _, team := range teams {
			apps, err := c.repo.ChartsForTeamGet(ctx, team.ID)
			if err != nil {
				c.log.WithError(err).Error("problem retrieving apps for teams")
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err})
				return
			}
			events, err := c.repo.EventsByOwnerGet(ctx, team.ID, 5)
			if err != nil {
				c.log.WithError(err).Error("problem retrieving apps for teams")
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err})
				return
			}

			teamApps[team.ID] = teamInfo{
				Team:      team,
				Namespace: k8s.TeamIDToNamespace(team.ID),
				Apps:      apps,
				Events:    events,
			}
		}

		ctx.HTML(http.StatusOK, "admin/index", gin.H{
			"errors":     flashes,
			"teams":      teamApps,
			"gcpProject": c.gcpProject,
			"loggedIn":   ctx.GetBool(middlewares.LoggedInKey),
			"admin":      ctx.GetBool(middlewares.AdminKey),
		})
	})

	c.router.GET("/admin/:chart", func(ctx *gin.Context) {
		chartType := getChartType(ctx.Param("chart"))

		values, err := c.repo.GlobalValuesGet(ctx, chartType)
		if err != nil {
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, "/admin")
				return
			}

			ctx.Redirect(http.StatusSeeOther, "/admin")
			return
		}

		session := sessions.Default(ctx)
		flashes := session.Flashes()
		err = session.Save()
		if err != nil {
			c.log.WithError(err).Error("problem saving session")
			ctx.Redirect(http.StatusSeeOther, "/admin")
			return
		}

		ctx.HTML(http.StatusOK, "admin/chart", gin.H{
			"values":   values,
			"errors":   flashes,
			"chart":    string(chartType),
			"loggedIn": ctx.GetBool(middlewares.LoggedInKey),
			"admin":    ctx.GetBool(middlewares.AdminKey),
		})
	})

	c.router.POST("/admin/:chart", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		chartType := getChartType(ctx.Param("chart"))

		err := ctx.Request.ParseForm()
		if err != nil {
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, "admin")
				return
			}
			ctx.Redirect(http.StatusSeeOther, "admin")
			return
		}

		changedValues, err := c.findGlobalValueChanges(ctx, ctx.Request.PostForm, chartType)
		if err != nil {
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
				return
			}

			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
			return
		}

		if len(changedValues) == 0 {
			session.AddFlash("Ingen endringer lagret")
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
				return
			}
			ctx.Redirect(http.StatusSeeOther, "/admin")
			return
		}

		gob.Register(changedValues)
		session.AddFlash(changedValues)
		err = session.Save()
		if err != nil {
			c.log.WithError(err).Error("problem saving session")
			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
			return
		}

		ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v/confirm", chartType))
	})

	c.router.GET("/admin/:chart/confirm", func(ctx *gin.Context) {
		chartType := getChartType(ctx.Param("chart"))
		session := sessions.Default(ctx)
		changedValues := session.Flashes()
		err := session.Save()
		if err != nil {
			c.log.WithError(err).Error("problem saving session")
			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
			return
		}

		ctx.HTML(http.StatusOK, "admin/confirm", gin.H{
			"changedValues": changedValues,
			"chart":         string(chartType),
			"loggedIn":      ctx.GetBool(middlewares.LoggedInKey),
			"admin":         ctx.GetBool(middlewares.AdminKey),
		})
	})

	c.router.POST("/admin/:chart/confirm", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		chartType := getChartType(ctx.Param("chart"))

		err := ctx.Request.ParseForm()
		if err != nil {
			c.log.WithError(err)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v/confirm", chartType))
				return
			}
			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v/confirm", chartType))
			return
		}

		if err := c.updateGlobalValues(ctx, ctx.Request.PostForm, chartType); err != nil {
			c.log.WithError(err)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
				return
			}
			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
			return
		}

		if err != nil {
			c.log.WithError(err)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v/confirm", chartType))
				return
			}
			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v/confirm", chartType))
			return
		}

		ctx.Redirect(http.StatusSeeOther, "/admin")
	})

	c.router.POST("/admin/:chart/sync", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		chartType := getChartType(ctx.Param("chart"))
		team := ctx.PostForm("team")

		if err := c.syncChart(ctx, team, chartType); err != nil {
			c.log.WithError(err).Errorf("syncing %v", chartType)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
			}
		}

		ctx.Redirect(http.StatusSeeOther, "/admin")
	})

	c.router.POST("/admin/:chart/sync/all", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		chartType := getChartType(ctx.Param("chart"))

		if err := c.syncChartForAllTeams(ctx, chartType); err != nil {
			c.log.WithError(err).Errorf("resyncing all instances of %v", chartType)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
			}
		}

		ctx.Redirect(http.StatusSeeOther, "/admin")
	})

	c.router.POST("/admin/team/sync/all", func(ctx *gin.Context) {
		session := sessions.Default(ctx)

		if err := c.syncTeams(ctx); err != nil {
			c.log.WithError(err).Errorf("resyncing all teams")
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
			}
		}

		ctx.Redirect(http.StatusSeeOther, "/admin")
	})

	c.router.POST("/admin/team/:team/delete", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		slug := ctx.Param("team")

		team, err := c.repo.TeamBySlugGet(ctx, slug)
		if err != nil {
			c.log.WithError(err).Errorf("deleting team")
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
			}
		}

		if err := c.repo.RegisterDeleteTeamEvent(ctx, team.ID); err != nil {
			c.log.WithError(err).Errorf("registering delete team event")
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
			}
		}

		ctx.Redirect(http.StatusSeeOther, "/admin")
	})

	c.router.GET("/admin/event/:id", func(ctx *gin.Context) {
		header, err := c.getEvent(ctx)
		if err != nil {
			c.log.WithError(err).Errorf("getting event logs")
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
			}
			ctx.Redirect(http.StatusSeeOther, "/admin")
		}

		header["loggedIn"] = ctx.GetBool(middlewares.LoggedInKey)
		header["admin"] = ctx.GetBool(middlewares.AdminKey)

		ctx.HTML(http.StatusOK, "admin/event", header)
	})

	c.router.POST("/admin/event/:id", func(ctx *gin.Context) {
		err := c.setEventStatus(ctx)
		if err != nil {
			c.log.WithError(err).Errorf("setting event status")
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
			}
			ctx.Redirect(http.StatusSeeOther, "/admin/event/"+ctx.Param("id"))
		}

		ctx.Redirect(http.StatusSeeOther, "/admin/event/"+ctx.Param("id"))
	})
}

func (c *client) syncTeams(ctx context.Context) error {
	teams, err := c.repo.TeamsGet(ctx)
	if err != nil {
		return err
	}

	for _, team := range teams {
		err := c.repo.RegisterUpdateTeamEvent(ctx, team)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *client) syncChartForAllTeams(ctx context.Context, chartType gensql.ChartType) error {
	teams, err := c.repo.TeamsForChartGet(ctx, chartType)
	if err != nil {
		return err
	}

	for _, teamID := range teams {
		err := c.syncChart(ctx, teamID, chartType)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *client) syncChart(ctx context.Context, teamID string, chartType gensql.ChartType) error {
	switch chartType {
	case gensql.ChartTypeJupyterhub:
		values := chart.JupyterConfigurableValues{
			TeamID: teamID,
		}
		return c.repo.RegisterUpdateJupyterEvent(ctx, teamID, values)
	case gensql.ChartTypeAirflow:
		values := chart.AirflowConfigurableValues{
			TeamID: teamID,
		}
		return c.repo.RegisterUpdateAirflowEvent(ctx, teamID, values)
	}

	return nil
}

func (c *client) findGlobalValueChanges(ctx context.Context, formValues url.Values, chartType gensql.ChartType) (map[string]diffValue, error) {
	originals, err := c.repo.GlobalValuesGet(ctx, chartType)
	if err != nil {
		return nil, err
	}

	changed := findChangedValues(originals, formValues)
	findDeletedValues(changed, originals, formValues)

	return changed, nil
}

func (c *client) updateGlobalValues(ctx context.Context, formValues url.Values, chartType gensql.ChartType) error {
	for key, values := range formValues {
		if values[0] == "" {
			err := c.repo.GlobalValueDelete(ctx, key, chartType)
			if err != nil {
				return err
			}
		} else {
			value, encrypted, err := c.parseValue(values)
			if err != nil {
				return err
			}

			err = c.repo.GlobalChartValueInsert(ctx, key, value, encrypted, chartType)
			if err != nil {
				return err
			}
		}
	}

	return c.syncChartForAllTeams(ctx, chartType)
}

func (c *client) parseValue(values []string) (string, bool, error) {
	if len(values) == 2 {
		value, err := c.repo.EncryptValue(values[0])
		if err != nil {
			return "", false, err
		}
		return value, true, nil
	}

	return values[0], false, nil
}

func findDeletedValues(changedValues map[string]diffValue, originals []gensql.ChartGlobalValue, formValues url.Values) {
	for _, original := range originals {
		notFound := true
		for key := range formValues {
			if original.Key == key {
				notFound = false
				break
			}
		}

		if notFound {
			changedValues[original.Key] = diffValue{
				Old: original.Value,
			}
		}
	}
}

func findChangedValues(originals []gensql.ChartGlobalValue, formValues url.Values) map[string]diffValue {
	changedValues := map[string]diffValue{}

	for key, values := range formValues {
		var encrypted string
		value := values[0]
		if len(values) == 2 {
			encrypted = values[1]
		}

		if strings.HasPrefix(key, "key") {
			correctValue := valueForKey(changedValues, key)
			if correctValue != nil {
				changedValues[value] = *correctValue
				delete(changedValues, key)
			} else {
				key := strings.Replace(key, "key", "value", 1)
				diff := diffValue{
					New:       key,
					Encrypted: encrypted,
				}
				changedValues[value] = diff
			}
		} else if strings.HasPrefix(key, "value") {
			correctKey := keyForValue(changedValues, key)
			if correctKey != "" {
				diff := diffValue{
					New:       value,
					Encrypted: encrypted,
				}
				changedValues[correctKey] = diff
			} else {
				key := strings.Replace(key, "value", "key", 1)
				diff := diffValue{
					New:       value,
					Encrypted: encrypted,
				}
				changedValues[key] = diff
			}
		} else {
			for _, originalValue := range originals {
				if originalValue.Key == key {
					if originalValue.Value != value {
						diff := diffValue{
							Old:       originalValue.Value,
							New:       value,
							Encrypted: encrypted,
						}
						changedValues[key] = diff
						break
					}
				}
			}
		}
	}

	return changedValues
}

func valueForKey(values map[string]diffValue, needle string) *diffValue {
	for key, value := range values {
		if key == needle {
			return &value
		}
	}

	return nil
}

func keyForValue(values map[string]diffValue, needle string) string {
	for key, value := range values {
		if value.New == needle {
			return key
		}
	}

	return ""
}

func (c *client) getEvent(ctx *gin.Context) (gin.H, error) {
	eventID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		return gin.H{}, err
	}

	event, err := c.repo.EventGet(ctx, eventID)
	if err != nil {
		return gin.H{}, err
	}

	eventLogs, err := c.repo.EventLogsForEventGet(ctx, eventID)
	if err != nil {
		return gin.H{}, err
	}

	return gin.H{
		"event": event,
		"logs":  eventLogs,
	}, nil
}

func (c *client) setEventStatus(ctx *gin.Context) error {
	eventID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		return err
	}

	var status database.EventStatus
	switch ctx.PostForm("status") {
	case string(database.EventStatusNew):
		status = database.EventStatusNew
	case string(database.EventStatusFailed):
		status = database.EventStatusFailed
	default:
		return fmt.Errorf("invalid status %v", ctx.PostForm("status"))
	}

	return c.repo.EventSetStatus(ctx, eventID, status)
}
