package service

import "skiverleih/internal/domain"

// Die Repository-Interfaces beschreiben, was der Service vom Speicher braucht.
// store.MemoryStore erfüllt sie; ein anderer Speicher (z. B. eine DB) könnte sie
// später ebenfalls erfüllen, ohne dass sich der Service ändert.

type SkiRepository interface {
	CreateSki(domain.Ski) domain.Ski
	UpdateSki(domain.Ski) error
	Ski(id int) (domain.Ski, error)
	ListSkis() []domain.Ski
	DeleteSki(id int) error
}

type PersonRepository interface {
	CreatePerson(domain.Person) domain.Person
	UpdatePerson(domain.Person) error
	Person(id int) (domain.Person, error)
	ListPersons() []domain.Person
	DeletePerson(id int) error
}

type RentalRepository interface {
	CreateRental(domain.Rental) domain.Rental
	UpdateRental(domain.Rental) error
	Rental(id int) (domain.Rental, error)
	ListRentals() []domain.Rental
	DeleteRental(id int) error
}

// Repository fasst alle drei zusammen.
type Repository interface {
	SkiRepository
	PersonRepository
	RentalRepository
}
