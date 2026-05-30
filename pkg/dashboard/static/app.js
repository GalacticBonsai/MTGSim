	function showTab(tabId, btn) {
		document.querySelectorAll('.tab-content').forEach(el => el.classList.remove('active'));
		document.querySelectorAll('.tab-btn').forEach(el => el.classList.remove('active'));
		document.getElementById(tabId).classList.add('active');
		if (btn) {
			btn.classList.add('active');
		} else {
			const button = document.querySelector('.tab-btn[data-tab="' + tabId + '"]');
			if (button) button.classList.add('active');
		}
	}

	// Cross-tab shared state
	let activeDeckName = localStorage.getItem('mtgsim_active_deck') || '';
	let globalCardLibraryMap = {};
	let unimplementedCards = new Set();
	let ignoreRecSelectChange = false;

	function setActiveDeck(deckName) {
		activeDeckName = deckName;
		if (deckName) {
			localStorage.setItem('mtgsim_active_deck', deckName);
		} else {
			localStorage.removeItem('mtgsim_active_deck');
		}
		syncActiveDeckDropdowns();
	}

	function syncActiveDeckDropdowns() {
		const recSelect = document.getElementById('recDeckSelect');
		if (!recSelect || !activeDeckName) return;
		if (recSelect.value === activeDeckName) return; // nothing to do

		ignoreRecSelectChange = true;
		recSelect.value = activeDeckName;
		ignoreRecSelectChange = false;

		// Only auto-load recommendations if the user is actually on that tab
		const recTab = document.getElementById('recommendations');
		if (recTab && recTab.classList.contains('active')) {
			loadRecommendations();
		}
	}

	function goToRecommendations() {
		const deckName = activeDeckName || (selectedDeck && selectedDeck.deck_name);
		if (!deckName) {
			alert('Select a deck in Test Results first');
			return;
		}
		showTab('recommendations');
		const recSelect = document.getElementById('recDeckSelect');
		if (recSelect) {
			ignoreRecSelectChange = true;
			recSelect.value = deckName;
			ignoreRecSelectChange = false;
			loadRecommendations();
		}
	}

	// Wire up the recommendations dropdown change handler programmatically
	// so we can suppress it during auto-refreshes.
	function setupRecDeckSelectHandler() {
		const recSelect = document.getElementById('recDeckSelect');
		if (!recSelect) return;
		recSelect.onchange = function() {
			if (ignoreRecSelectChange) {
				console.log('[RecSelect] Ignored programmatic change');
				return;
			}
			console.log('[RecSelect] User changed to:', recSelect.value);
			loadRecommendations();
		};
	}

	function goToMyDeck() {
		const deckName = activeDeckName || (selectedDeck && selectedDeck.deck_name);
		if (!deckName) {
			alert('Select a deck in Test Results first');
			return;
		}
		showTab('my-deck');
		const deck = allEDHDecksList.find(d => d.deck_name === deckName);
		if (deck) {
			document.getElementById('deckName').value = getDeckDisplayName(deck.deck_name);
		}

		// Try to load saved deck contents from localStorage
		const savedName = localStorage.getItem('mtgsim_deck_name');
		const savedList = localStorage.getItem('mtgsim_deck_list');
		if (savedName && savedList && normalizeUploadedName(savedName) === normalizeUploadedName(deckName)) {
			document.getElementById('deckList').value = savedList;
			addCardToDeck();
			return;
		}

		// Fallback: reconstruct a partial deck list from card_stats if available
		if (selectedDeck && selectedDeck.card_stats && Object.keys(selectedDeck.card_stats).length > 0) {
			let lines = [];
			for (let cardName of Object.keys(selectedDeck.card_stats)) {
				lines.push('1 ' + cardName);
			}
			if (lines.length > 0) {
				document.getElementById('deckList').value = lines.join('\n');
				addCardToDeck();
			}
		}
	}

	function addCardToMyDeck(cardName) {
		const deckList = document.getElementById('deckList');
		const current = deckList.value.trim();
		deckList.value = current ? current + '\n1 ' + cardName : '1 ' + cardName;
		updateDeckEditorStats();
		const myDeckBtn = document.querySelector('.tab-btn[data-tab="my-deck"]');
		if (myDeckBtn) {
			myDeckBtn.style.background = '#4ecdc4';
			myDeckBtn.style.color = '#000';
			setTimeout(() => {
				myDeckBtn.style.background = '';
				myDeckBtn.style.color = '';
			}, 600);
		}
	}

	let currentEDHData = null;
	let edhSortState = { key: 'win_rate', dir: 'desc' };
	let cardLibrarySortKey = 'winRate';
	let cardLibrarySortAsc = false;
	let selectedDeck = null;
	let allEDHDecksList = [];
	let selectedDeckCardsSortKey = 'winRate';
	let selectedDeckCardsSortAsc = false;
	
	// Track uploaded decks (persists across page reloads)
	let uploadedDeckNames = new Set();
	
	// Load uploaded deck names from localStorage on startup
	function loadUploadedDecks() {
		const stored = localStorage.getItem('mtgsim_uploaded_decks');
		const names = stored ? JSON.parse(stored) : [];
		uploadedDeckNames = new Set(names.map(normalizeUploadedName));
	}
	
	// Save uploaded deck names to localStorage
	function saveUploadedDecks() {
		localStorage.setItem('mtgsim_uploaded_decks', JSON.stringify(Array.from(uploadedDeckNames)));
	}

	// Helper function to extract filename from path
	function getDeckDisplayName(fullPath) {
		if (!fullPath) return '';
		return fullPath.split('/').pop().split('\\').pop();
	}

	function normalizeUploadedName(name) {
		return name.replace(/\.(dck|txt|dec)$/i, '');
	}

	function isUploadedDeck(deck) {
		if (!deck || !deck.deck_name) return false;
		const deckName = normalizeUploadedName(deck.deck_name);
		return Array.from(uploadedDeckNames).some(name => {
			const norm = normalizeUploadedName(name);
			return deckName.includes(norm) ||
				deckName.endsWith(norm) ||
				getDeckDisplayName(deckName) === norm ||
				getDeckDisplayName(deck.deck_name) === norm;
		});
	}

	function sortEDH(key) {
		if (!currentEDHData || !currentEDHData.decks) return;
		if (edhSortState.key === key) {
			edhSortState.dir = edhSortState.dir === 'asc' ? 'desc' : 'asc';
		} else {
			edhSortState.key = key;
			edhSortState.dir = 'asc';
		}
		document.querySelectorAll('#edhDecks .th-sort').forEach(th => {
			th.classList.remove('asc', 'desc');
			if (th.dataset.sortKey === key) {
				th.classList.add(edhSortState.dir);
			}
		});
		renderEDH(currentEDHData);
	}

	async function loadResults() {
		try {
			const res = await fetch('/api/results');
			const data = await res.json();
			renderSummary(data);
			renderWinRateChart(data);
			renderTopDecksChart(data);
		} catch (err) {
			console.error('Error loading results:', err);
		}
		try {
			const res = await fetch('/api/edh-results');
			const data = await res.json();
			if (data.enabled) {
				document.getElementById('edhSummarySection').style.display = '';
				renderEDHSummary(data.summary || {});
				renderEDH(data);
				renderTopCards(data);
			}
		} catch (err) {
			console.error('Error loading EDH results:', err);
		}
		try {
			const res = await fetch('/api/card-library');
			const data = await res.json();
			if (data.enabled) {
				renderCardLibrary(data.cards || {});
			}
		} catch (err) {
			console.error('Error loading card library:', err);
		}

	}

	function renderSummary(data) {
		let html = '';
		html += '<div class="card" title="Total number of 1v1 games simulated"><h2>Total Games</h2><div class="value">' + (data.total_games || 0) + '</div></div>';
		html += '<div class="card" title="Number of distinct decklists tested"><h2>Unique Decks</h2><div class="value">' + (data.unique_decks || 0) + '</div></div>';
		document.getElementById('summary').innerHTML = html;
	}

	function renderDecks(data) {
		const decks = data.decks || [];

		let html = '';

		if (data.truncated) {
			html += '<tr><td colspan="4" style="color:#888;">'
				+ 'Showing first '
				+ decks.length
				+ ' of '
				+ data.totalDecks
				+ ' decks'
				+ '</td></tr>';
		}

		for (let d of decks) {
			html += '<tr title="' + d.name + ': ' + d.wins + 'W / ' + d.losses + 'L (' + (d.win_rate || 0).toFixed(1) + '% win rate)"><td>' + d.name + '</td><td>' + d.wins + '</td><td>' + d.losses + '</td><td><strong>' + (d.win_rate || 0).toFixed(1) + '%</strong></td></tr>';
		}

		document.getElementById('decksBody').innerHTML =
			html || '<tr><td colspan="4">No data</td></tr>';
	}

	function renderEDH(data) {
		currentEDHData = data;
		allEDHDecksList = data.decks || [];
		filterEDHDecks();
	}

	function filterEDHDecks() {
		let decks = (allEDHDecksList || []).slice();
		const search = (document.getElementById('deckSearch') ? document.getElementById('deckSearch').value : '').toLowerCase();
		const onlyUploaded = document.getElementById('showOnlyUploaded') ? document.getElementById('showOnlyUploaded').checked : false;

		// Filter to uploaded decks only
		if (onlyUploaded) {
			decks = decks.filter(d => isUploadedDeck(d));
		}

		// Filter by search term
		if (search) {
			decks = decks.filter(d =>
				(d.deck_name || '').toLowerCase().includes(search) ||
				(d.commander_name || '').toLowerCase().includes(search)
			);
		}
		
		// Sort
		const key = edhSortState.key;
		const dir = edhSortState.dir;
		const numericKeys = ['games','wins','losses','win_rate','avg_final_life','commander_damage_kos','life_loss_kos','mill_kos','avg_commander_casts','avg_mana_spent','avg_cards_played','avg_lands_played','avg_spells_cast','avg_creatures_cast','avg_combat_damage','max_storm_count','eliminations','avg_mulligans'];
		const isNum = numericKeys.includes(key);
		decks.sort((a, b) => {
			let av = a[key] !== undefined ? a[key] : (isNum ? 0 : '');
			let bv = b[key] !== undefined ? b[key] : (isNum ? 0 : '');
			if (typeof av === 'string') av = av.toLowerCase();
			if (typeof bv === 'string') bv = bv.toLowerCase();
			if (av < bv) return dir === 'asc' ? -1 : 1;
			if (av > bv) return dir === 'asc' ? 1 : -1;
			return 0;
		});
		
		document.querySelectorAll('#edhDecks .th-sort').forEach(th => {
			th.classList.remove('asc', 'desc');
			if (th.dataset.sortKey === key) {
				th.classList.add(dir);
			}
		});
		
		let html = '';
		for (let d of decks) {
			let displayName = getDeckDisplayName(d.deck_name);
			let tooltip = d.deck_name + ' (' + (d.commander_name || 'No Commander') + ') — ' + d.games + ' pods, ' + d.wins + 'W / ' + d.losses + 'L';
			let selected = selectedDeck && selectedDeck.deck_name === d.deck_name ? ' style="background:#3a3a3a;"' : '';
			let escapedDeckName = d.deck_name.replace(/\\/g, "\\\\").replace(/'/g, "\\'");
			html += '<tr title="' + tooltip + '" onclick="selectDeck(' + "'" + escapedDeckName + "'" + ')"' + selected + ' style="cursor:pointer;">'
				+ '<td>' + displayName + '</td>'
				+ '<td>' + (d.commander_name || '-') + '</td>'
				+ '<td>' + d.games + '</td>'
				+ '<td>' + d.wins + '</td>'
				+ '<td>' + d.losses + '</td>'
				+ '<td><strong>' + (d.win_rate || 0).toFixed(1) + '%</strong></td>'
				+ '<td>' + (d.avg_final_life || 0).toFixed(1) + '</td>'
				+ '<td>' + d.commander_damage_kos + '</td>'
				+ '<td>' + d.life_loss_kos + '</td>'
				+ '<td>' + d.mill_kos + '</td>'
				+ '<td>' + (d.avg_commander_casts || 0).toFixed(2) + '</td>'
				+ '<td>' + (d.avg_mana_spent || 0).toFixed(1) + '</td>'
				+ '<td>' + (d.avg_cards_played || 0).toFixed(1) + '</td>'
				+ '<td>' + (d.avg_lands_played || 0).toFixed(1) + '</td>'
				+ '<td>' + (d.avg_spells_cast || 0).toFixed(1) + '</td>'
				+ '<td>' + (d.avg_creatures_cast || 0).toFixed(1) + '</td>'
				+ '<td>' + (d.avg_combat_damage || 0).toFixed(1) + '</td>'
				+ '<td>' + (d.max_storm_count || 0) + '</td>'
				+ '<td>' + (d.eliminations || 0) + '</td>'
				+ '<td>' + (d.avg_mulligans || 0).toFixed(2) + '</td>'
				+ '</tr>';
		}
		document.getElementById('edhDecksBody').innerHTML = html || '<tr><td colspan="20" style="color:#888;">No decks match the current filter. Upload a deck or uncheck "Show only my uploaded decks".</td></tr>';
	}

	function selectDeck(deckName) {
		const deck = allEDHDecksList.find(d => d.deck_name === deckName);
		if (!deck) return;

		selectedDeck = deck;
		setActiveDeck(deckName);
		document.getElementById('selectedDeckStats').style.display = '';
		document.getElementById('clearDeckSelection').style.display = 'inline-block';

		// Update stats
		document.getElementById('selectedDeckName').textContent = getDeckDisplayName(deck.deck_name);
		document.getElementById('selectedDeckCommander').textContent = deck.commander_name || 'Unknown';
		document.getElementById('selectedDeckGames').textContent = deck.games;
		document.getElementById('selectedDeckWinRate').textContent = (deck.win_rate || 0).toFixed(1) + '%';
		document.getElementById('selectedDeckAvgMana').textContent = (deck.avg_mana_spent || 0).toFixed(1);

		// Show card stats
		renderSelectedDeckCards();

		// Re-render deck list to highlight selection
		filterEDHDecks();
	}

	function clearSelectedDeck() {
		selectedDeck = null;
		document.getElementById('selectedDeckStats').style.display = 'none';
		document.getElementById('clearDeckSelection').style.display = 'none';
		filterEDHDecks();
	}

	function sortSelectedDeckCards(key) {
		if (selectedDeckCardsSortKey === key) {
			selectedDeckCardsSortAsc = !selectedDeckCardsSortAsc;
		} else {
			selectedDeckCardsSortKey = key;
			selectedDeckCardsSortAsc = false;
		}
		renderSelectedDeckCards();
	}

	function renderSelectedDeckCards() {
		if (!selectedDeck) return;
		
		const cardStats = selectedDeck.card_stats || {};
		let cards = [];
		
		for (let [name, stats] of Object.entries(cardStats)) {
			if (stats.casts >= 3) {  // minimum 3 casts
				cards.push({
					name: name,
					casts: stats.casts,
					wins: stats.wins || 0,
					winRate: stats.casts > 0 ? ((stats.wins || 0) / stats.casts * 100) : 0
				});
			}
		}
		
		// Sort
		cards.sort((a, b) => {
			let av = a[selectedDeckCardsSortKey];
			let bv = b[selectedDeckCardsSortKey];
			
			if (typeof av === 'string') av = av.toLowerCase();
			if (typeof bv === 'string') bv = bv.toLowerCase();
			
			if (av < bv) return selectedDeckCardsSortAsc ? -1 : 1;
			if (av > bv) return selectedDeckCardsSortAsc ? 1 : -1;
			return 0;
		});
		
		let html = '';
		for (let c of cards.slice(0, 100)) {
			html += '<tr title="' + c.name + ': ' + c.casts + ' casts, ' + c.wins + ' wins">'
				+ '<td>' + c.name + '</td>'
				+ '<td>' + c.casts + '</td>'
				+ '<td>' + c.wins + '</td>'
				+ '<td><strong>' + c.winRate.toFixed(1) + '%</strong></td>'
				+ '</tr>';
		}
		
		document.getElementById('selectedDeckCardsBody').innerHTML = html || '<tr><td colspan="4">No cards with 3+ casts</td></tr>';
	}

	function renderTopCardsForCommander() {
		 const commander = document.getElementById("commanderSelect").value;

		let decks = allEDHDecks;

		if (commander) {
			decks = decks.filter(d => d.commander_name === commander);
		}

		let cards = [];

		for (let d of decks) {
			let cs = d.card_stats || {};
			for (let [name, perf] of Object.entries(cs)) {
			if (perf.casts >= 5) {
				cards.push({
				deck: getDeckDisplayName(d.deck_name),
				name,
				casts: perf.casts,
				wins: perf.wins,
				winRate: (perf.wins / perf.casts) * 100
				});
			}
			}
		}

		cards.sort((a, b) => b.winRate - a.winRate || b.casts - a.casts);
		cards = cards.slice(0, 100);

		let html = '';

		for (let c of cards) {
			let img = c.image_url
				? '<img src="' + c.image_url + '" height="40" style="vertical-align:middle;margin-right:8px;border-radius:4px;">'
				: '';
			const escapedName = String(c.name).replace(/['\\]/g, "\\$&");

			let tooltip =
				c.name + ' in ' + c.deck +
				': ' + c.casts + ' casts, ' + c.wins + ' wins';

			html +=
				'<tr title="' + tooltip + '">' +
					'<td>' + c.deck + '</td>' +
					'<td>' + img + c.name + '</td>' +
					'<td>' + c.casts + '</td>' +
					'<td>' + c.wins + '</td>' +
					'<td><strong>' + c.winRate.toFixed(1) + '%</strong></td>' +
					'<td><button onclick="addCardToMyDeck(\'' + escapedName + '\')" style="padding:4px 10px; background:#4ecdc4; color:#000; border:none; border-radius:4px; cursor:pointer; font-size:0.8em; font-weight:bold;">+ Add</button></td>' +
				'</tr>';
		}

		document.getElementById('topCardsBody').innerHTML =
			html || '<tr><td colspan="6">No cards for this commander</td></tr>';
	}
		let allEDHDecks = [];

		function renderTopCards(data) {
		allEDHDecks = data.decks || [];
		allEDHDecksList = data.decks || [];
		populateCommanderDropdown(allEDHDecks);
		populateRecommendationsDeckSelect();
		renderTopCardsForCommander();
		}
		function populateCommanderDropdown(decks) {
			const select = document.getElementById("commanderSelect");

			const commanders = [...new Set(
				decks.map(d => d.commander_name).filter(Boolean)
			)].sort();

			select.innerHTML = '<option value="">All Commanders</option>' +
				commanders.map(c => '<option value="' + c + '">' + c + '</option>').join("");
		}
		function filterCommanders() {
			const q = document.getElementById("commanderSearch").value.toLowerCase();
			const select = document.getElementById("commanderSelect");

			for (let opt of select.options) {
				if (!opt.value) continue;
				opt.style.display = opt.value.toLowerCase().includes(q) ? "" : "none";
			}
		}
		

		function renderEDHSummary(s) {
			let html = '';
			html += '<div class="card" title="Total number of EDH pods simulated"><h2>EDH Pods</h2><div class="value">' + (s.total_games || 0) + '</div></div>';
			html += '<div class="card" title="Average number of turns per pod"><h2>Average Turns</h2><div class="value">' + (s.average_turns || 0).toFixed(1) + '</div></div>';
			html += '<div class="card" title="Highest storm count reached in any pod"><h2>Highest Storm</h2><div class="value">' + (s.highest_storm_count || 0) + '</div></div>';
			html += '<div class="card" title="Total mana spent by all players across all pods"><h2>Total Mana Spent</h2><div class="value">' + (s.total_mana_spent || 0) + '</div><span class="unit">avg ' + (s.average_mana_spent || 0).toFixed(1) + '/pod</span></div>';
			html += '<div class="card" title="Total cards played by all players across all pods"><h2>Total Cards Played</h2><div class="value">' + (s.total_cards_played || 0) + '</div><span class="unit">avg ' + (s.average_cards_played || 0).toFixed(1) + '/pod</span></div>';
			html += '<div class="card" title="Total combat damage dealt across all pods"><h2>Combat Damage</h2><div class="value">' + (s.total_combat_damage || 0) + '</div><span class="unit">avg ' + (s.average_combat_damage || 0).toFixed(1) + '/pod</span></div>';
			html += '<div class="card" title="Total player eliminations across all pods"><h2>Eliminations</h2><div class="value">' + (s.total_eliminations || 0) + '</div><span class="unit">avg ' + (s.average_eliminations || 0).toFixed(1) + '/pod</span></div>';
			document.getElementById('edhSummary').innerHTML = html;
		}

		function renderCardLibrary(cards) {
			currentCardLibraryRows = [];
			globalCardLibraryMap = {};

			for (let [name, perf] of Object.entries(cards)) {
				const winRate = perf.casts > 0 ? (perf.wins / perf.casts * 100) : 0;
				currentCardLibraryRows.push({
					name: name,
					casts: perf.casts || 0,
					wins: perf.wins || 0,
					winRate: winRate,
					image_url: perf.image_url || ''
				});
				globalCardLibraryMap[name.toLowerCase()] = { casts: perf.casts || 0, wins: perf.wins || 0, winRate: winRate, image_url: perf.image_url || '' };
			}

			// IMPORTANT: ensure UI renders even if inputs are missing
			if (!document.getElementById('cardLibrarySearch')) {
				document.getElementById('cardLibraryBody').innerHTML =
					'<tr><td colspan="4">Loading controls...</td></tr>';
				return;
			}

			filterCardLibrary();
		}
		
		function sortCardLibrary(key) {
			if (cardLibrarySortKey === key) {
				cardLibrarySortAsc = !cardLibrarySortAsc;
			} else {
				cardLibrarySortKey = key;
				cardLibrarySortAsc = false;
			}

			filterCardLibrary();
		}

		function filterCardLibrary() {
			let rows = [...currentCardLibraryRows];

			const searchEl = document.getElementById('cardLibrarySearch');
			const minEl = document.getElementById('cardLibraryMinCasts');
			const limitEl = document.getElementById('cardLibraryLimit');

			const search = (searchEl ? searchEl.value : '').toLowerCase();
			const minCasts = parseInt(minEl ? minEl.value : '5');
			const limit = parseInt(limitEl ? limitEl.value : '100');

			rows = rows.filter(r => {
				return r.casts >= minCasts &&
					r.name.toLowerCase().includes(search);
			});

			rows.sort((a, b) => {
				let av = a[cardLibrarySortKey];
				let bv = b[cardLibrarySortKey];

				if (typeof av === 'string') av = av.toLowerCase();
				if (typeof bv === 'string') bv = bv.toLowerCase();

				if (av < bv) return cardLibrarySortAsc ? -1 : 1;
				if (av > bv) return cardLibrarySortAsc ? 1 : -1;
				return 0;
			});

			const total = rows.length;

			rows = rows.slice(0, limit);

			let html = '';

			for (let c of rows) {
				let img = '';
				const escapedName = c.name.replace(/'/g, "\\'");

				if (c.image_url) {
					img =
						'<div class="card-thumb-wrapper">' +
							'<img src="' + c.image_url + '" class="card-thumb">' +
							'<div class="card-preview">' +
								'<img src="' + c.image_url + '" class="card-preview-img">' +
							'</div>' +
						'</div>';
				}

				let tooltip =
					c.name + ': ' +
					c.casts + ' casts, ' +
					c.wins + ' wins (' +
					c.winRate.toFixed(1) + '% win rate)';

				html +=
					'<tr title="' + tooltip + '">' +
						'<td>' +
							'<div style="display:flex;align-items:center;gap:12px;">' +
								img +
								'<span>' + c.name + '</span>' +
							'</div>' +
						'</td>' +
						'<td>' + c.casts + '</td>' +
						'<td>' + c.wins + '</td>' +
						'<td><strong>' + c.winRate.toFixed(1) + '%</strong></td>' +
						'<td><button onclick="addCardToMyDeck(\'' + escapedName + '\')" style="padding:4px 10px; background:#4ecdc4; color:#000; border:none; border-radius:4px; cursor:pointer; font-size:0.8em; font-weight:bold;">+ Add</button></td>' +
					'</tr>';
			}

			document.getElementById('cardLibraryBody').innerHTML =
				html || '<tr><td colspan="5">No matching cards</td></tr>';

			document.getElementById('cardLibraryMeta').textContent =
				'Showing ' + rows.length + ' of ' + total + ' matching cards';
		}

		async function loadEDHGames() {
			try {
				const res = await fetch('/api/edh-games');
				const data = await res.json();
				if (data.enabled) renderEDHGames(data.games || []);
			} catch (err) {
				console.error('Error loading EDH games:', err);
		}
		}

		function renderEDHGames(games) {
			let html = '';
			for (let g of games) {
				const players = (g.Players || []).map(p => p.DeckName + ' mana=' + (p.ManaSpent || 0) + ' cards=' + (p.CardsPlayed || 0) + (p.Eliminated ? ' ✗' : ' ✓')).join('<br>');
				const events = g.Events || [];
				const last = events.length ? events[events.length - 1].kind : '-';
				let tooltip = 'Pod winner: ' + (g.Winner || 'Draw') + ', Turns: ' + g.Turns + ', Storm: ' + (g.MaxStormCount || 0);
				html += '<tr title="' + tooltip + '"><td>' + (g.Winner || 'Draw') + '</td><td>' + g.Turns + '</td><td>' + (g.MaxStormCount || 0) + '</td><td>' + (g.TotalManaSpent || 0) + '</td><td>' + (g.TotalCardsPlayed || 0) + '</td><td>' + (g.TotalCombatDamage || 0) + '</td><td>' + players + '</td><td>' + events.length + '</td><td>' + last + '</td></tr>';
		}
			document.getElementById('edhGamesBody').innerHTML = html || '<tr><td colspan="9">No recent pods</td></tr>';
		}

	// Chart.js instances
	let winRateChart = null;
	let topDecksChart = null;

	function renderWinRateChart(data) {
		const decks = data.decks || [];
		
		// Group decks by win rate ranges
		const ranges = {
			'0-25%': 0,
			'25-50%': 0,
			'50-75%': 0,
			'75-100%': 0
		};
		
		for (let d of decks) {
			const wr = d.win_rate || 0;
			if (wr < 25) ranges['0-25%']++;
			else if (wr < 50) ranges['25-50%']++;
			else if (wr < 75) ranges['50-75%']++;
			else ranges['75-100%']++;
		}
		
		const ctx = document.getElementById('winRateChart');
		if (!ctx) return;
		
		// Destroy existing chart if it exists
		if (winRateChart) winRateChart.destroy();
		
		winRateChart = new Chart(ctx, {
			type: 'doughnut',
			data: {
				labels: Object.keys(ranges),
				datasets: [{
					data: Object.values(ranges),
					backgroundColor: [
						'#e74c3c',
						'#f39c12',
						'#3498db',
						'#2ecc71'
					],
					borderColor: '#1a1a1a',
					borderWidth: 2
				}]
			},
			options: {
				responsive: true,
				maintainAspectRatio: false,
				plugins: {
					legend: {
						position: 'right',
						labels: {
							color: '#ccc',
							font: { size: 12 }
						}
					}
				}
			}
		});
	}

	function renderTopDecksChart(data) {
		const decks = (data.decks || []).slice();
		
		// Sort by win rate and take top 8
		decks.sort((a, b) => (b.win_rate || 0) - (a.win_rate || 0));
		const topDecks = decks.slice(0, 8);
		
		const labels = topDecks.map(d => d.name.replace(/\.deck$/, '').substring(0, 15));
		const winRates = topDecks.map(d => (d.win_rate || 0).toFixed(1));
		
		const ctx = document.getElementById('topDecksChart');
		if (!ctx) return;
		
		// Destroy existing chart if it exists
		if (topDecksChart) topDecksChart.destroy();
		
		topDecksChart = new Chart(ctx, {
			type: 'bar',
			data: {
				labels: labels,
				datasets: [{
					label: 'Win Rate %',
					data: winRates,
					backgroundColor: topDecks.map((d, i) => {
						const wr = d.win_rate || 0;
						if (wr >= 60) return '#2ecc71';
						if (wr >= 50) return '#3498db';
						if (wr >= 40) return '#f39c12';
						return '#e74c3c';
					}),
					borderColor: '#ddd',
					borderWidth: 1
				}]
			},
			options: {
				indexAxis: 'y',
				responsive: true,
				maintainAspectRatio: false,
				plugins: {
					legend: {
						labels: {
							color: '#ccc'
						}
					}
				},
				scales: {
					x: {
						max: 100,
						ticks: {
							color: '#888'
						},
						grid: {
							color: '#333'
						}
					},
					y: {
						ticks: {
							color: '#888'
						},
						grid: {
							display: false
						}
					}
				}
			}
		});
	}

	// Game control functions
	async function runMoreGames() {
		const countInput = document.getElementById('gamesInput');
		const count = Math.max(1, Math.min(10000, parseInt(countInput.value) || 100));
		
		const btn = document.getElementById('runGamesBtn');
		btn.disabled = true;
		btn.textContent = 'Starting...';
		
		try {
			const res = await fetch('/api/run-games?count=' + count, {
				method: 'POST'
			});
			
			const data = await res.json();
			
			if (res.ok) {
				// Update status immediately after starting
				await updateGameStatus();
			} else {
				btn.textContent = '▶ Run Games';
				btn.disabled = false;
				const errorMsg = data.error || 'Unknown error';
				alert('Error starting games: ' + errorMsg);
				console.error('Server error:', data);
			}
		} catch (err) {
			console.error('Error running games:', err);
			btn.textContent = '▶ Run Games';
			btn.disabled = false;
			alert('Network error: ' + err.message);
		}
	}

	async function updateGameStatus() {
		try {
			const res = await fetch('/api/game-status');
			const data = await res.json();
			const indicator = document.getElementById('gameStatusIndicator');
			const btn = document.getElementById('runGamesBtn');
			
			if (data.running) {
				indicator.textContent = '🔄 Games running...';
				indicator.style.color = '#4ecdc4';
				btn.disabled = true;
				btn.textContent = '🔄 Running...';
			} else {
				indicator.textContent = '✓ Ready';
				indicator.style.color = '#888';
				btn.disabled = false;
				btn.textContent = '▶ Run Games';
			}
		} catch (err) {
			console.error('Error checking game status:', err);
		}
	}

	async function uploadDeck() {
		const fileInput = document.getElementById('deckFileInput');
		const file = fileInput.files[0];
		if (!file) {
			alert('Please select a deck file');
			return;
		}

		// Read file contents into the deck editor so it can be edited immediately
		try {
			const text = await file.text();
			const baseName = file.name.replace(/\.(dck|txt|dec)$/i, '');
			document.getElementById('deckName').value = baseName;
			document.getElementById('deckList').value = text;
			addCardToDeck();
			saveDeckEditorState();
		} catch (err) {
			console.error('Error reading deck file:', err);
			alert('Could not read deck file contents');
			return;
		}

		const formData = new FormData();
		formData.append('deck', file);

		const btn = document.getElementById('uploadDeckBtn');
		const status = document.getElementById('suggestedDeckStatus');
		btn.disabled = true;
		status.textContent = 'Uploading...';

		try {
			const res = await fetch('/api/upload-deck', {
				method: 'POST',
				body: formData
			});

			const data = await res.json();

			if (res.ok) {
				// Track this deck as user-uploaded (normalized name without extension)
				uploadedDeckNames.add(normalizeUploadedName(file.name));
				saveUploadedDecks(); // Persist to localStorage

				status.textContent = '✓ ' + file.name + ' uploaded';
				status.style.color = '#4ecdc4';
				fileInput.value = '';

				// Refresh recommendations dropdown to include this deck
				populateRecommendationsDeckSelect();
			} else {
				alert('Upload failed: ' + (data.error || 'Unknown error'));
				status.textContent = '✗ Upload failed';
				status.style.color = '#ff6b6b';
			}
		} catch (err) {
			console.error('Error uploading deck:', err);
			alert('Network error: ' + err.message);
			status.textContent = '✗ Network error';
			status.style.color = '#ff6b6b';
		} finally {
			btn.disabled = false;
		}
	}

	async function uploadOverviewDeck() {
		const fileInput = document.getElementById('overviewDeckFileInput');
		const file = fileInput.files[0];
		if (!file) {
			alert('Please select a deck file');
			return;
		}

		// Also load into the My Deck editor
		try {
			const text = await file.text();
			const baseName = file.name.replace(/\.(dck|txt|dec)$/i, '');
			document.getElementById('deckName').value = baseName;
			document.getElementById('deckList').value = text;
			addCardToDeck();
			saveDeckEditorState();
		} catch (err) {
			console.error('Error reading overview deck file:', err);
		}

		const formData = new FormData();
		formData.append('deck', file);

		const btn = document.getElementById('overviewUploadDeckBtn');
		const status = document.getElementById('overviewSuggestedDeckStatus');
		btn.disabled = true;
		status.textContent = 'Uploading...';

		try {
			const res = await fetch('/api/upload-deck', {
				method: 'POST',
				body: formData
			});

			const data = await res.json();

			if (res.ok) {
				uploadedDeckNames.add(normalizeUploadedName(file.name));
				saveUploadedDecks();
				status.textContent = '✓ ' + file.name + ' uploaded';
				status.style.color = '#4ecdc4';
				fileInput.value = '';
				populateRecommendationsDeckSelect();
			} else {
				alert('Upload failed: ' + (data.error || 'Unknown error'));
				status.textContent = '✗ Upload failed';
				status.style.color = '#ff6b6b';
			}
		} catch (err) {
			console.error('Error uploading overview deck:', err);
			alert('Network error: ' + err.message);
			status.textContent = '✗ Network error';
			status.style.color = '#ff6b6b';
		} finally {
			btn.disabled = false;
		}
	}

	// ===== NEW FEATURES =====

	// Recommendations Tab
	const _originalLoadRecommendations = async function() {
		const deckSelect = document.getElementById('recDeckSelect');
		const deckName = deckSelect.value;
		if (!deckName) return;

		const recContent = document.getElementById('recContent');
		recContent.innerHTML = '<div class="loading">Loading recommendations...</div>';

		try {
			const res = await fetch('/api/card-recommendations?deck=' + encodeURIComponent(deckName));
			const data = await res.json();
			if (!data.enabled) {
				recContent.innerHTML = '<div class="error">Recommendations not available</div>';
				return;
			}

			const recs = data.recommendations;
			let html = '';

			// Header with deck info and quick actions
			const deck = allEDHDecksList.find(d => d.deck_name === deckName);
			if (deck) {
				html += '<div style="margin-bottom:20px; padding:15px; background:#1a1a1a; border-radius:6px; border-left:4px solid #5a6dd8;">';
				html += '<h4 style="margin-top:0; color:#5a6dd8;">' + getDeckDisplayName(deck.deck_name) + '</h4>';
				html += '<div style="color:#888; margin-bottom:10px;">Win Rate: <strong>' + (deck.win_rate || 0).toFixed(1) + '%</strong> • Games: ' + deck.games + ' • Commander: ' + (deck.commander_name || '-') + '</div>';
				html += '<button onclick="goToMyDeck()" style="padding:6px 12px; background:#4ecdc4; color:#000; border:none; border-radius:4px; cursor:pointer; font-weight:bold; font-size:0.9em;">🛠️ Open in My Deck</button>';
				html += '</div>';
			}

			if (recs.remove_candidates && recs.remove_candidates.length > 0) {
				html += '<div style="margin-bottom:20px; padding:15px; background:#1a1a1a; border-radius:6px; border-left:4px solid #e74c3c;">';
				html += '<h4 style="margin-top:0; color:#e74c3c;">🗑️ Cards to Remove</h4>';
				html += '<p style="color:#888; margin:10px 0;">These cards have below-average performance in this deck:</p>';
				html += '<div>';
				for (let c of recs.remove_candidates) {
					const escaped = c.card_name.replace(/'/g, "\\'");
					html += '<div style="margin-bottom:8px; padding:8px; background:#0a0e27; border-radius:4px; display:flex; justify-content:space-between; align-items:center;">';
					html += '<div><strong>' + c.card_name + '</strong> ';
					html += '<span style="color:#ff6b6b;">Win Rate: ' + c.win_rate.toFixed(1) + '%</span> ';
					html += '<span style="color:#888;">(' + c.casts + ' casts)</span></div>';
					html += '<button onclick="addCardToMyDeck(\'' + escaped + '\')" style="padding:4px 10px; background:#ff6b6b; color:#fff; border:none; border-radius:4px; cursor:pointer; font-size:0.8em; font-weight:bold; margin-left:8px; opacity:0.7;" title="Add to My Deck anyway">+</button>';
					html += '</div>';
				}
				html += '</div></div>';
			}

			if (recs.add_candidates && recs.add_candidates.length > 0) {
				html += '<div style="margin-bottom:20px; padding:15px; background:#1a1a1a; border-radius:6px; border-left:4px solid #2ecc71;">';
				html += '<h4 style="margin-top:0; color:#2ecc71;">➕ Cards to Test</h4>';
				html += '<p style="color:#888; margin:10px 0;">These cards perform well globally and might improve this deck:</p>';
				html += '<div>';
				for (let c of recs.add_candidates.slice(0, 10)) {
					const escaped = c.card_name.replace(/'/g, "\\'");
					html += '<div style="margin-bottom:8px; padding:8px; background:#0a0e27; border-radius:4px; display:flex; justify-content:space-between; align-items:center;">';
					html += '<div><strong>' + c.card_name + '</strong> ';
					html += '<span style="color:#2ecc71;">Win Rate: ' + c.win_rate.toFixed(1) + '%</span> ';
					html += '<span style="color:#888;">(' + c.casts + ' casts globally)</span></div>';
					html += '<button onclick="addCardToMyDeck(\'' + escaped + '\')" style="padding:4px 10px; background:#4ecdc4; color:#000; border:none; border-radius:4px; cursor:pointer; font-size:0.8em; font-weight:bold; margin-left:8px;">+ Add</button>';
					html += '</div>';
				}
				html += '</div></div>';
			}

			if (html === '') {
				html = '<div class="error">No recommendations available for this deck</div>';
			}

			recContent.innerHTML = html;
		} catch (err) {
			console.error('Error loading recommendations:', err);
			recContent.innerHTML = '<div class="error">Error loading recommendations</div>';
		}
	};

	// Diagnostic wrapper — logs every call so we can trace unwanted refreshes
	async function loadRecommendations() {
		const deckSelect = document.getElementById('recDeckSelect');
		console.log('[loadRecommendations] triggered for deck:', deckSelect ? deckSelect.value : 'N/A');
		console.trace();
		return _originalLoadRecommendations();
	}

	// Populate recommendations deck select - show all available EDH decks
	async function populateRecommendationsDeckSelect() {
		const select = document.getElementById('recDeckSelect');
		if (!select) return;

		// Suppress change events while we rebuild so removing/restoring options
		// doesn't spuriously fire loadRecommendations() and flash the panel.
		ignoreRecSelectChange = true;

		// Remember what the user had selected before we clear anything
		const previousValue = select.value;

		// Clear existing options except first placeholder
		while (select.options.length > 1) {
			select.remove(1);
		}

		if (!allEDHDecksList || allEDHDecksList.length === 0) {
			ignoreRecSelectChange = false;
			return; // Wait for data to load
		}

		// Filter to only uploaded decks
		let uploadedDecks = allEDHDecksList.filter(d => isUploadedDeck(d)
		).sort((a, b) =>
			(getDeckDisplayName(a.deck_name) || '').localeCompare(getDeckDisplayName(b.deck_name) || '')
		);

		// Add uploaded decks to dropdown (display by filename only)
		for (let d of uploadedDecks) {
			const opt = document.createElement('option');
			opt.value = d.deck_name;
			opt.textContent = getDeckDisplayName(d.deck_name);
			select.appendChild(opt);
		}

		// Restore the user's selection if that option still exists,
		// otherwise fall back to the active deck
		if (previousValue && Array.from(select.options).some(o => o.value === previousValue)) {
			select.value = previousValue;
		} else if (activeDeckName) {
			select.value = activeDeckName;
		}

		ignoreRecSelectChange = false;

		// Only auto-load if the value actually changed and the Recommendations tab is visible
		const recTab = document.getElementById('recommendations');
		if (select.value !== previousValue && recTab && recTab.classList.contains('active')) {
			loadRecommendations();
		}
	}

	// Deck Editor
	let currentDeckList = [];

	function addCardToDeck() {
		const input = document.getElementById('deckList');
		const cards = input.value.split('\n')
			.map(line => {
				const match = line.match(/^(\d+)?\s*(.+)$/);
				if (!match) return null;
				const count = parseInt(match[1]) || 1;
				const name = match[2].trim();
				return { count, name };
			})
			.filter(c => c && c.name);
		
		currentDeckList = cards;
		updateDeckStats();
	}

	function updateDeckStats() {
		let totalCards = 0;
		let landCount = 0;
		let uniqueCards = new Set();

		for (let card of currentDeckList) {
			totalCards += card.count;
			uniqueCards.add(card.name);
			if (card.name.toLowerCase().includes('land') || card.name.match(/^(plains|island|swamp|mountain|forest)$/i)) {
				landCount += card.count;
			}
		}

		document.getElementById('deckCardCount').textContent = totalCards;
		document.getElementById('deckUniqueCount').textContent = uniqueCards.size;
		document.getElementById('deckLandCount').textContent = landCount;
		document.getElementById('deckAvgMana').textContent = '~' + (totalCards > 0 ? (totalCards / uniqueCards.size).toFixed(1) : '0');
		updateDeckEditorStats();
	}

	async function loadImplementationStatus() {
		try {
			const res = await fetch('/api/implementation');
			const data = await res.json();
			if (data.enabled && data.report && data.report.unimplemented_cards) {
				unimplementedCards = new Set(data.report.unimplemented_cards.map(c => c.name.toLowerCase()));
				// Refresh deck editor if cards are already loaded
				updateDeckEditorStats();
			}
		} catch (err) {
			console.error('Error loading implementation status:', err);
		}
	}

	function isCardImplemented(cardName) {
		return !unimplementedCards.has(cardName.toLowerCase());
	}

	function updateDeckEditorStats() {
		const container = document.getElementById('deckEditorCardStats');
		if (!container || currentDeckList.length === 0) {
			if (container) container.innerHTML = '';
			return;
		}
		let html = '<div style="font-size:0.9em; color:#888; margin-bottom:6px;">Card performance from global library:</div>';
		let found = 0;
		for (let card of currentDeckList) {
			const lib = globalCardLibraryMap[card.name.toLowerCase()];
			const implWarning = isCardImplemented(card.name) ? '' : ' <span style="color:#e74c3c; font-weight:bold;" title="This card is not fully implemented in the simulator">⚠️</span>';
			if (lib) {
				found++;
				const wrColor = lib.winRate >= 55 ? '#2ecc71' : lib.winRate >= 45 ? '#f39c12' : '#e74c3c';
				html += '<div style="display:flex; justify-content:space-between; padding:4px 0; border-bottom:1px solid #1a1f3a;">';
				html += '<span>' + card.count + 'x ' + card.name + implWarning + '</span>';
				html += '<span style="color:' + wrColor + ';">' + lib.winRate.toFixed(1) + '% WR (' + lib.casts + ' casts)</span>';
				html += '</div>';
			} else {
				html += '<div style="display:flex; justify-content:space-between; padding:4px 0; border-bottom:1px solid #1a1f3a;">';
				html += '<span>' + card.count + 'x ' + card.name + implWarning + '</span>';
				html += '<span style="color:#666;">no data</span>';
				html += '</div>';
			}
		}
		if (found === 0 && currentDeckList.length > 0) {
			html += '<div style="color:#666; font-size:0.9em;">No performance data for these cards yet. Run more games or search in Card Analysis.</div>';
		}
		container.innerHTML = html;
	}

	function saveDeckEditorState() {
		const name = document.getElementById('deckName').value;
		const list = document.getElementById('deckList').value;
		localStorage.setItem('mtgsim_deck_name', name);
		localStorage.setItem('mtgsim_deck_list', list);
	}

	function loadDeckEditorState() {
		const name = localStorage.getItem('mtgsim_deck_name');
		const list = localStorage.getItem('mtgsim_deck_list');
		if (name !== null) document.getElementById('deckName').value = name;
		if (list !== null) {
			document.getElementById('deckList').value = list;
			addCardToDeck();
		}
	}

	function clearDeckEditor() {
		document.getElementById('deckName').value = '';
		document.getElementById('deckList').value = '';
		currentDeckList = [];
		updateDeckStats();
		localStorage.removeItem('mtgsim_deck_name');
		localStorage.removeItem('mtgsim_deck_list');
	}

	async function testDeckComposition() {
		const deckName = document.getElementById('deckName').value || 'test-deck';
		const content = document.getElementById('deckList').value;
		if (!content.trim()) {
			alert('Please enter some cards');
			return;
		}
		const filename = deckName.endsWith('.txt') || deckName.endsWith('.dck') || deckName.endsWith('.dec') ? deckName : deckName + '.txt';
		const blob = new Blob([content], { type: 'text/plain' });
		const file = new File([blob], filename, { type: 'text/plain' });
		const formData = new FormData();
		formData.append('deck', file);
		const btn = document.getElementById('testDeckBtn');
		if (btn) { btn.disabled = true; btn.textContent = 'Uploading...'; }
		try {
			const res = await fetch('/api/upload-deck', { method: 'POST', body: formData });
			const data = await res.json();
			if (res.ok) {
				uploadedDeckNames.add(normalizeUploadedName(filename));
				saveUploadedDecks();
				saveDeckEditorState();
				alert('✓ ' + filename + ' uploaded. Go to Overview to run games with it.');
				populateRecommendationsDeckSelect();
			} else {
				alert('Upload failed: ' + (data.error || 'Unknown error'));
			}
		} catch (err) {
			alert('Network error: ' + err.message);
		} finally {
			if (btn) { btn.disabled = false; btn.textContent = 'Test Deck Composition'; }
		}
	}

	// Card Search
	async function performCardSearch() {
		const query = document.getElementById('cardSearchInput').value.trim();
		const sortBy = document.getElementById('cardSortSelect').value;
		const resultsDiv = document.getElementById('cardSearchResults');

		if (!query) {
			resultsDiv.innerHTML = '<div class="loading">Start searching for cards...</div>';
			return;
		}

		resultsDiv.innerHTML = '<div class="loading">Searching...</div>';

		try {
			const res = await fetch('/api/card-search?q=' + encodeURIComponent(query));
			const data = await res.json();

			if (!data.enabled) {
				resultsDiv.innerHTML = '<div class="error">Card search not available</div>';
				return;
			}

			let results = data.results || [];

			// Sort results
			if (sortBy === 'win_rate') {
				results.sort((a, b) => (b.win_rate || 0) - (a.win_rate || 0));
			} else if (sortBy === 'casts') {
				results.sort((a, b) => (b.casts || 0) - (a.casts || 0));
			} else if (sortBy === 'wins') {
				results.sort((a, b) => (b.wins || 0) - (a.wins || 0));
			} else {
				results.sort((a, b) => (a.name || '').localeCompare(b.name || ''));
			}

			let html = '';
			for (let card of results.slice(0, 50)) {
				const wr = card.win_rate || 0;
				const wrColor = wr >= 55 ? '#2ecc71' : wr >= 45 ? '#f39c12' : '#e74c3c';
				const escapedName = card.name.replace(/'/g, "\\'");

				html += '<div style="padding:12px; background:#1a1a1a; border-radius:6px; border-left:4px solid ' + wrColor + ';">';
				if (card.image) {
					html += '<img src="' + card.image + '" style="width:100%; border-radius:4px; margin-bottom:8px;" alt="' + card.name + '">';
				}
				html += '<div><strong>' + card.name + '</strong></div>';
				html += '<div style="color:#888; font-size:0.9em; margin-top:4px;">';
				html += card.casts + ' casts • ' + card.wins + ' wins';
				html += '</div>';
				html += '<div style="color:' + wrColor + '; font-weight:bold; margin-top:4px;">';
				html += 'Win Rate: ' + wr.toFixed(1) + '%';
				html += '</div>';
				html += '<button onclick="addCardToMyDeck(\'' + escapedName + '\')" style="margin-top:8px; padding:6px 12px; background:#4ecdc4; color:#000; border:none; border-radius:4px; cursor:pointer; font-size:0.85em; font-weight:bold;">+ Add to My Deck</button>';
				html += '</div>';
			}

			if (results.length === 0) {
				html = '<div class="error" style="grid-column:1/-1;">No cards found matching "' + query + '"</div>';
			}

			resultsDiv.innerHTML = html;
		} catch (err) {
			console.error('Error searching cards:', err);
			resultsDiv.innerHTML = '<div class="error">Error searching cards</div>';
		}
	}

	// Matchup Matrix
	let matchupMatrixData = [];

	async function loadMatchupMatrix() {
		const bodyEl = document.getElementById('matchupBody');
		if (!bodyEl) return;

		bodyEl.innerHTML = '<tr><td colspan="7" class="loading">Loading...</td></tr>';

		try {
			const res = await fetch('/api/matchup-matrix');
			const data = await res.json();

			if (!data.enabled) {
				bodyEl.innerHTML = '<tr><td colspan="7" class="error">Matchup data not available</td></tr>';
				return;
			}

			matchupMatrixData = data.decks || [];
			renderMatchupMatrix(matchupMatrixData);
		} catch (err) {
			console.error('Error loading matchups:', err);
			bodyEl.innerHTML = '<tr><td colspan="7" class="error">Error loading matchup data</td></tr>';
		}
	}

	function renderMatchupMatrix(decks) {
		const bodyEl = document.getElementById('matchupBody');
		if (!bodyEl) return;
		let html = '';

		for (let d of decks) {
			const wrColor = d.win_rate >= 55 ? '#2ecc71' : d.win_rate >= 45 ? '#f39c12' : '#e74c3c';
			const escapedName = d.name.replace(/'/g, "\\'");
			html += '<tr>';
			html += '<td>' + getDeckDisplayName(d.name) + '</td>';
			html += '<td>' + d.commander + '</td>';
			html += '<td style="color:' + wrColor + '; font-weight:bold;">' + (d.win_rate || 0).toFixed(1) + '%</td>';
			html += '<td>' + d.games + '</td>';
			html += '<td>' + Math.round(d.games * (d.win_rate || 0) / 100) + '</td>';
			html += '<td>' + Math.round(d.games * (1 - (d.win_rate || 0) / 100)) + '</td>';
			html += '<td><button onclick="selectDeck(\'' + escapedName + '\'); goToRecommendations();" style="padding:4px 10px; background:#5a6dd8; color:#fff; border:none; border-radius:4px; cursor:pointer; font-size:0.8em; font-weight:bold;">🔮 Recs</button></td>';
			html += '</tr>';
		}

		bodyEl.innerHTML = html || '<tr><td colspan="7">No matchup data available</td></tr>';
	}

	function filterMatchupMatrix() {
		const q = (document.getElementById('matchupSearch')?.value || '').toLowerCase();
		if (!q) {
			renderMatchupMatrix(matchupMatrixData);
			return;
		}
		const filtered = matchupMatrixData.filter(d =>
			(d.name && d.name.toLowerCase().includes(q)) ||
			(d.commander && d.commander.toLowerCase().includes(q))
		);
		renderMatchupMatrix(filtered);
	}

	// Meta Snapshots
	async function saveSnapshot() {
		const nameInput = document.getElementById('snapshotName');
		const name = nameInput.value || '';
		
		try {
			const res = await fetch('/api/save-snapshot?name=' + encodeURIComponent(name), {
				method: 'POST'
			});
			const data = await res.json();

			if (data.success) {
				alert('✓ Snapshot saved at ' + new Date(data.timestamp * 1000).toLocaleString());
				nameInput.value = '';
				loadSnapshotsList();
			} else {
				alert('Error saving snapshot: ' + (data.error || 'Unknown error'));
			}
		} catch (err) {
			console.error('Error saving snapshot:', err);
			alert('Network error: ' + err.message);
		}
	}

	async function loadSnapshotsList() {
		const listEl = document.getElementById('snapshotsList');
		if (!listEl) return;

		try {
			const res = await fetch('/api/snapshots');
			const data = await res.json();

			if (!data.enabled) {
				listEl.innerHTML = '<div class="error">Snapshots not available</div>';
				return;
			}

			const snapshots = data.snapshots || [];
			let html = '';

			if (snapshots.length === 0) {
				html = '<div style="color:#888; padding:15px;">No snapshots saved yet. Save one to get started!</div>';
			} else {
				html = '<table class="table" style="font-size:0.95em;"><thead><tr><th>Name</th><th>Date</th><th>Decks</th><th>Avg WR</th></tr></thead><tbody>';
				for (let snap of snapshots) {
					const date = new Date(snap.timestamp).toLocaleString();
					html += '<tr>';
					html += '<td>' + snap.name + '</td>';
					html += '<td style="color:#888;">' + date + '</td>';
					html += '<td>' + snap.deck_count + '</td>';
					html += '<td><strong>' + (snap.average_wr || 0).toFixed(1) + '%</strong></td>';
					html += '</tr>';
				}
				html += '</tbody></table>';
			}

			listEl.innerHTML = html;
		} catch (err) {
			console.error('Error loading snapshots:', err);
			listEl.innerHTML = '<div class="error">Error loading snapshots</div>';
		}
	}

	async function loadSnapshotComparison() {
		const analysisEl = document.getElementById('snapshotAnalysis');
		if (!analysisEl) return;

		analysisEl.innerHTML = '<div class="loading">Loading comparison...</div>';

		try {
			const res = await fetch('/api/snapshot-comparison');
			const data = await res.json();

			if (data.error) {
				analysisEl.innerHTML = '<div class="error">' + data.error + '</div>';
				return;
			}

			const comp = data.comparison;
			let html = '<h4>Meta Changes</h4>';

			// Deck win rate changes
			if (comp.deck_win_rate_changes && Object.keys(comp.deck_win_rate_changes).length > 0) {
				html += '<div style="margin-bottom:20px;">';
				html += '<h5>Win Rate Changes</h5>';
				
				const changes = Object.entries(comp.deck_win_rate_changes)
					.sort((a, b) => Math.abs(b[1]) - Math.abs(a[1]))
					.slice(0, 10);
				
				for (let [deck, change] of changes) {
					const color = change > 0 ? '#2ecc71' : change < 0 ? '#e74c3c' : '#888';
					const arrow = change > 0 ? '📈' : change < 0 ? '📉' : '→';
					html += '<div style="padding:8px; background:#0a0e27; border-radius:4px; margin-bottom:6px;">';
					html += arrow + ' <strong>' + getDeckDisplayName(deck) + '</strong>: ';
					html += '<span style="color:' + color + '; font-weight:bold;">' + (change > 0 ? '+' : '') + change.toFixed(2) + '%</span>';
					html += '</div>';
				}
				html += '</div>';
			}

			// New decks
			if (comp.new_decks && comp.new_decks.length > 0) {
				html += '<div style="margin-bottom:20px;">';
				html += '<h5>New Decks</h5>';
				for (let deck of comp.new_decks.slice(0, 5)) {
					html += '<div style="padding:8px; background:#0a0e27; border-radius:4px; margin-bottom:6px;">➕ ' + getDeckDisplayName(deck) + '</div>';
				}
				html += '</div>';
			}

			// Removed decks
			if (comp.removed_decks && comp.removed_decks.length > 0) {
				html += '<div style="margin-bottom:20px;">';
				html += '<h5>Removed Decks</h5>';
				for (let deck of comp.removed_decks.slice(0, 5)) {
					html += '<div style="padding:8px; background:#0a0e27; border-radius:4px; margin-bottom:6px;">❌ ' + getDeckDisplayName(deck) + '</div>';
				}
				html += '</div>';
			}

			// Card performance shifts
			if (comp.card_performance_shift && Object.keys(comp.card_performance_shift).length > 0) {
				html += '<div style="margin-bottom:20px;">';
				html += '<h5>📈 Card Performance Shifts</h5>';
				const shifts = Object.entries(comp.card_performance_shift)
					.sort((a, b) => Math.abs(b[1].change || 0) - Math.abs(a[1].change || 0))
					.slice(0, 10);
				for (let [cardName, shift] of shifts) {
					const change = shift.change || 0;
					const color = change > 0 ? '#2ecc71' : change < 0 ? '#e74c3c' : '#888';
					const arrow = change > 0 ? '📈' : change < 0 ? '📉' : '→';
					html += '<div style="padding:8px; background:#0a0e27; border-radius:4px; margin-bottom:6px; display:flex; justify-content:space-between; align-items:center;">';
					html += '<div>' + arrow + ' <strong>' + cardName + '</strong>: ';
					html += '<span style="color:' + color + '; font-weight:bold;">' + (change > 0 ? '+' : '') + change.toFixed(1) + '%</span>';
					html += ' <span style="color:#888; font-size:0.85em;">(' + (shift.casts || 0) + ' casts)</span></div>';
					const escaped = cardName.replace(/'/g, "\\'");
					html += '<button onclick="addCardToMyDeck(\'' + escaped + '\')" style="padding:4px 10px; background:#4ecdc4; color:#000; border:none; border-radius:4px; cursor:pointer; font-size:0.8em; font-weight:bold;">+ Add</button>';
					html += '</div>';
				}
				html += '</div>';
			}

			if (html === '<h4>Meta Changes</h4>') {
				html = '<div class="error">No significant changes between snapshots</div>';
			}

			analysisEl.innerHTML = html;
		} catch (err) {
			console.error('Error loading comparison:', err);
			analysisEl.innerHTML = '<div class="error">Error loading comparison</div>';
		}
	}

	async function loadMetaTrends() {
		const analysisEl = document.getElementById('snapshotAnalysis');
		if (!analysisEl) return;

		analysisEl.innerHTML = '<div class="loading">Loading trends...</div>';

		try {
			const res = await fetch('/api/meta-trends');
			const data = await res.json();

			if (!data.enabled) {
				analysisEl.innerHTML = '<div class="error">Trend data not available</div>';
				return;
			}

			const trends = data.trends || [];
			if (trends.length === 0) {
				analysisEl.innerHTML = '<div class="error">No trend data available. Save multiple snapshots to see trends.</div>';
				return;
			}

			let html = '<h4>Meta Trends Over Time</h4>';

			for (let trend of trends.slice(0, 5)) {
				const date = new Date(trend.timestamp).toLocaleString();
				html += '<div style="margin-bottom:20px; padding:15px; background:#0a0e27; border-radius:6px;">';
				html += '<div style="color:#888; margin-bottom:10px;"><strong>📅 ' + date + '</strong></div>';
				html += '<div>Decks in Meta: <strong>' + trend.diversity + '</strong></div>';
				html += '<div>Average WR: <strong>' + (trend.average_win_rate || 0).toFixed(1) + '%</strong></div>';
				html += '<div>WR Range: ' + (trend.lowest_win_rate || 0).toFixed(1) + '% - ' + (trend.highest_win_rate || 0).toFixed(1) + '%</div>';
				
				if (trend.top_decks && trend.top_decks.length > 0) {
					html += '<div style="margin-top:10px;"><strong>Top 5 Decks:</strong>';
					for (let i = 0; i < Math.min(5, trend.top_decks.length); i++) {
						const d = trend.top_decks[i];
						html += '<div style="padding:4px 0; margin-left:12px; color:#4ecdc4;">' + (i+1) + '. ' + getDeckDisplayName(d.name) + ' (' + (d.win_rate || 0).toFixed(1) + '%)</div>';
					}
					html += '</div>';
				}
				html += '</div>';
			}

			analysisEl.innerHTML = html;
		} catch (err) {
			console.error('Error loading trends:', err);
			analysisEl.innerHTML = '<div class="error">Error loading trends</div>';
		}
	}

	// Initialize - load persistent state first
	loadUploadedDecks();
	loadDeckEditorState();
	loadImplementationStatus();
	setupRecDeckSelectHandler();

	// Load snapshots on page load
	setInterval(loadSnapshotsList, 10000);
	loadSnapshotsList();

	setInterval(loadResults, 5000);
	setInterval(updateGameStatus, 2000);
	setInterval(loadEDHGames, 5000);
	setInterval(loadMatchupMatrix, 5000);
	loadResults();
	loadEDHGames();
	loadMatchupMatrix();

	// After initial load, try to restore active deck selection across tabs
	setTimeout(() => {
		if (activeDeckName) syncActiveDeckDropdowns();
	}, 1000);

	// Self-test: simulate auto-refresh cycles and assert the Recommendations
	// dropdown doesn't flip to a different deck.
	window.testRecSelectStability = async function(iterations = 3, delayMs = 100) {
		const select = document.getElementById('recDeckSelect');
		if (!select) {
			console.error('[Test] recDeckSelect not found');
			return false;
		}

		// Pick a non-empty value to test with (skip placeholder)
		if (select.options.length <= 1) {
			console.error('[Test] No decks in dropdown yet. Run a simulation first.');
			return false;
		}

		// Remember whatever is currently selected (or pick the last option)
		let testValue = select.value || select.options[select.options.length - 1].value;
		select.value = testValue;
		console.log('[Test] Starting stability test. Target value:', testValue);

		let passed = true;
		for (let i = 0; i < iterations; i++) {
			await new Promise(r => setTimeout(r, delayMs));
			await populateRecommendationsDeckSelect();
			await new Promise(r => setTimeout(r, delayMs));

			const after = select.value;
			if (after !== testValue) {
				console.error('[Test] FAIL on iteration', i + 1, ': expected', testValue, 'got', after);
				passed = false;
			} else {
				console.log('[Test] OK iteration', i + 1, ': value stayed', after);
			}
		}
		console.log('[Test] Result:', passed ? 'PASSED' : 'FAILED');
		return passed;
	};
