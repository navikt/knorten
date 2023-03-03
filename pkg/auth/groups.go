package auth

type Group struct {
	Name  string
	Email string
}

type Groups []Group

func (g Groups) Names() []string {
	names := []string{}
	for _, g := range g {
		names = append(names, g.Name)
	}
	return names
}

func (g Groups) Emails() []string {
	emails := []string{}
	for _, g := range g {
		emails = append(emails, g.Email)
	}
	return emails
}

func (g Groups) Get(email string) (Group, bool) {
	for _, grp := range g {
		if grp.Email == email {
			return grp, true
		}
	}

	return Group{}, false
}

func (g Groups) Contains(email string) bool {
	_, ok := g.Get(email)
	return ok
}
