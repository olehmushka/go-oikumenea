// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
)

// Ukrainian-style name pools. Patronymic (по-батькові) is stored in person_persons.given2 (there is
// no dedicated patronymic column). Marriage pairing uses `sex` — the seeder pairs male↔female only.

var maleGiven = []string{
	"Oleksandr", "Serhii", "Andrii", "Dmytro", "Volodymyr", "Mykola", "Ivan", "Petro", "Vasyl",
	"Yurii", "Bohdan", "Taras", "Roman", "Viktor", "Oleh", "Vitalii", "Maksym", "Denys", "Artem", "Ihor",
}
var femaleGiven = []string{
	"Olena", "Nataliia", "Iryna", "Tetiana", "Kateryna", "Oksana", "Yuliia", "Mariia", "Anna",
	"Svitlana", "Halyna", "Liudmyla", "Viktoriia", "Sofiia", "Yaroslava", "Nadiia", "Vira", "Olha", "Zoriana", "Daryna",
}

// fatherGiven feeds the patronymic (child's given2 = <father's given> + -ovych / -ivna).
var fatherGiven = []string{
	"Oleksandr", "Serhii", "Andrii", "Dmytro", "Volodymyr", "Mykola", "Ivan", "Petro", "Vasyl", "Yurii",
}

// surname base forms (male). Female forms adjust the -yi/-ov/-enko endings where natural.
var surnameBase = []string{
	"Shevchenko", "Kovalenko", "Bondarenko", "Tkachenko", "Kravchenko", "Melnyk", "Boiko", "Kovalchuk",
	"Marchenko", "Lysenko", "Rudenko", "Havryliuk", "Moroz", "Poliakov", "Savchenko", "Klymenko",
	"Petryk", "Danylyuk", "Zhuravel", "Sydorenko", "Tkachuk", "Palamarchuk", "Romaniuk", "Onyshchenko",
}

func patronymic(father string, male bool) string {
	if male {
		return father + "ovych"
	}
	return father + "ivna"
}

// makeName returns (given, given2/patronymic, surname, displayName) for a person.
func (s *seeder) makeName(male bool) (given, giv2, surname, display string) {
	if male {
		given = s.pick(maleGiven)
	} else {
		given = s.pick(femaleGiven)
	}
	giv2 = patronymic(s.pick(fatherGiven), male)
	surname = s.pick(surnameBase)
	if !male {
		surname += "a" // rough feminine surname form for -o/-k endings; fine for demo
	}
	display = fmt.Sprintf("%s %s %s", surname, given, giv2)
	return
}

// sexOf returns the ISO-5218 code.
func sexOf(male bool) string {
	if male {
		return "male"
	}
	return "female"
}
