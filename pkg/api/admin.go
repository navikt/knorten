package api

import (
	"encoding/gob"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/nais/knorten/pkg/database/gensql"
	"net/http"
	"net/url"
	"strings"
)

type diffValue struct {
	Old string
	New string
}

func (a *API) setupAdminRoutes() {
	a.router.GET("/admin", func(c *gin.Context) {
		session := sessions.Default(c)
		flashes := session.Flashes()
		err := session.Save()
		if err != nil {
			a.log.WithError(err).Error("problem saving session")
			return
		}

		c.HTML(http.StatusOK, "admin/index", gin.H{
			"errors": flashes,
		})
	})

	a.router.GET("/admin/:chart", func(c *gin.Context) {
		chartType := getChartType(c.Param("chart"))

		var values []gensql.ChartGlobalValue
		var err error
		switch chartType {
		case gensql.ChartTypeJupyterhub:
			values, err = a.repo.GlobalValuesGet(c, gensql.ChartTypeJupyterhub)
		case gensql.ChartTypeAirflow:
			values, err = a.repo.GlobalValuesGet(c, gensql.ChartTypeAirflow)
		}

		if err != nil {
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, "/admin")
				return
			}

			c.Redirect(http.StatusSeeOther, "/admin")
			return
		}

		session := sessions.Default(c)
		flashes := session.Flashes()
		err = session.Save()
		if err != nil {
			a.log.WithError(err).Error("problem saving session")
			return
		}

		c.HTML(http.StatusOK, "admin/chart", gin.H{
			"values": values,
			"errors": flashes,
			"chart":  string(chartType),
		})
	})

	a.router.POST("/admin/:chart", func(c *gin.Context) {
		session := sessions.Default(c)
		chartType := getChartType(c.Param("chart"))

		err := c.Request.ParseForm()
		if err != nil {
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, "admin")
				return
			}
			c.Redirect(http.StatusSeeOther, "admin")
			return
		}

		var chartValues []gensql.ChartGlobalValue
		switch chartType {
		case gensql.ChartTypeJupyterhub:
			chartValues, err = a.repo.GlobalValuesGet(c, gensql.ChartTypeJupyterhub)
		case gensql.ChartTypeAirflow:
			chartValues, err = a.repo.GlobalValuesGet(c, gensql.ChartTypeAirflow)
		}

		if err != nil {
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
				return
			}

			c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
			return
		}

		formValues := c.Request.PostForm
		changedValues := findChangedValues(chartValues, formValues)
		findDeletedValues(changedValues, chartValues, formValues)
		gob.Register(changedValues)

		if len(changedValues) == 0 {
			session.AddFlash("Ingen endringer lagret")
			err = session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
				return
			}
			c.Redirect(http.StatusSeeOther, "/admin")
			return
		}

		session.AddFlash(changedValues)
		err = session.Save()
		if err != nil {
			a.log.WithError(err).Error("problem saving session")
			c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
			return
		}
		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v/confirm", chartType))
		return
	})

	a.router.GET("/admin/:chart/confirm", func(c *gin.Context) {
		chartType := getChartType(c.Param("chart"))
		session := sessions.Default(c)
		changedValues := session.Flashes()
		err := session.Save()
		if err != nil {
			a.log.WithError(err).Error("problem saving session")
			c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
			return
		}

		c.HTML(http.StatusOK, "admin/confirm", gin.H{
			"changedValues": changedValues,
			"chart":         string(chartType),
		})
	})
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
		value := values[0]

		if strings.HasPrefix(key, "key") {
			correctValue := valueForKey(changedValues, key)
			if correctValue != "" {
				diff := diffValue{
					New: correctValue,
				}
				changedValues[value] = diff
				delete(changedValues, key)
			} else {
				key := strings.Replace(key, "key", "value", 1)
				diff := diffValue{
					New: key,
				}
				changedValues[value] = diff
			}
		} else if strings.HasPrefix(key, "value") {
			correctKey := keyForValue(changedValues, key)
			if correctKey != "" {
				diff := diffValue{
					New: value,
				}
				changedValues[correctKey] = diff
			} else {
				key := strings.Replace(key, "value", "key", 1)
				diff := diffValue{
					New: value,
				}
				changedValues[key] = diff
			}
		} else {
			for _, originalValue := range originals {
				if originalValue.Key == key {
					if originalValue.Value != value {
						diff := diffValue{
							Old: originalValue.Value,
							New: value,
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

func valueForKey(values map[string]diffValue, needle string) string {
	for key, value := range values {
		if key == needle {
			return value.New
		}
	}

	return ""
}

func keyForValue(values map[string]diffValue, needle string) string {
	for key, value := range values {
		if value.New == needle {
			return key
		}
	}

	return ""
}
