package main

import (
	"fmt"
	"regexp"
	"strconv"
)

type mana struct {
	w int // white
	u int // blue
	b int // black
	r int // red
	g int // green
	c int // colorless
	a int // any
	p int // Phyrexian(2 life)
	s int // snow
}

func (m *mana) total() int {
	return m.w + m.u + m.b + m.r + m.g + m.c + m.a + m.p + m.s
}

func ParseManaCost(cost string) mana {
	var pool mana
	re := regexp.MustCompile(`\{(\w+)\}`)
	matches := re.FindAllStringSubmatch(cost, -1)

	for _, match := range matches {
		value := match[1]
		if num, err := strconv.Atoi(value); err == nil {
			pool.a += num
		} else {
			switch value {
			case "X":
				pool.a++ // X is 1 for simplicity. Could be 0, but more usually 1+
			case "W":
				pool.w++
			case "U":
				pool.u++
			case "B":
				pool.b++
			case "R":
				pool.r++
			case "G":
				pool.g++
			case "C":
				pool.c++
			case "S":
				pool.s++
			}
		}
	}
	return pool
}

func (p *mana) sub(cost mana) {
	p.w -= cost.w
	p.u -= cost.u
	p.b -= cost.b
	p.r -= cost.r
	p.g -= cost.g
	p.c -= cost.c
	p.a -= cost.a
	p.s -= cost.s
}

func (p *mana) add(m mana) {
	p.w += m.w
	p.u += m.u
	p.b += m.b
	p.r += m.r
	p.g += m.g
	p.c += m.c
	p.a += m.a
	p.s += m.s
}

func (p *mana) pay(c mana) error {
	if p.total() < c.total() {
		return fmt.Errorf("not enough Mana")
	}

	p.w -= c.w
	p.u -= c.u
	p.b -= c.b
	p.r -= c.r
	p.g -= c.g
	p.c -= c.c
	p.s -= c.s

	// if no any color, early return
	if c.a == 0 {
		return nil
	}

	// Try to pay it from same manapool
	if p.w >= c.a {
		p.w -= c.a
	} else if p.u >= c.a {
		p.u -= c.a
	} else if p.b >= c.a {
		p.b -= c.a
	} else if p.r >= c.a {
		p.r -= c.a
	} else if p.g >= c.a {
		p.g -= c.a
	} else if p.a >= c.a {
		p.a -= c.a
	}

	// pay from residual mana
	for c.a > 0 {
		if p.w > 0 {
			p.w--
		} else if p.u > 0 {
			p.u--
		} else if p.b > 0 {
			p.b--
		} else if p.r > 0 {
			p.r--
		} else if p.g > 0 {
			p.g--
		} else if p.c > 0 {
			p.c--
		} else if p.a > 0 {
			p.a--
		} else {
			return fmt.Errorf("not enough Mana")
		}
		c.a--
	}
	return nil
}
