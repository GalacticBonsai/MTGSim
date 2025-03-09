# Code Citations

## License: unknown
https://github.com/callerobertsson/gotools/tree/d47621b3593d92b6d888025c0f7b914dfc37f920/jackblack/deck/deck.go

```
) {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(d.Cards), func(i, j int) {
		d.Cards[i], d.Cards[j] = d.Cards[j]
```


## License: unknown
https://github.com/luc65r/cards/tree/37f98ef3e0c0a2dae579c0d2720f954175c20038/card/deck.go

```
())
	rand.Shuffle(len(d.Cards), func(i, j int) {
		d.Cards[i], d.Cards[j] = d.Cards[j], d.Cards[i]
	})
}

func (d
```


## License: unknown
https://github.com/cidstein/super-brunfo/tree/db5617d45426a0f31921f827ed95b82cc09570e4/backend/card/entity/deck.go

```
(time.Now().UnixNano())
	rand.Shuffle(len(d.Cards), func(i, j int) {
		d.Cards[i], d.Cards[j] = d.Cards[j], d.Cards[
```

