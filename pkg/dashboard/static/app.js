	function showTab(tabId, btn) {
		document.querySelectorAll('.tab-content').forEach(el => el.classList.remove('active'));
		document.querySelectorAll('.tab-btn').forEach(el => el.classList.remove('active'));
		document.getElementById(tabId).classList.add('active');
		btn.classList.add('active');
	}

	let currentEDHData = null;
	let edhSortState = { key: 'win_rate', dir: 'desc' };
	let cardLibrarySortKey = 'winRate';
	let cardLibrarySortAsc = false;
	let selectedDeck = null;
	let allEDHDecksList = [];
	let selectedDeckCardsSortKey = 'winRate';
	let selectedDeckCardsSortAsc = false;

	// Helper function to extract filename from path
	function getDeckDisplayName(fullPath) {
		if (!fullPath) return '';
		return fullPath.split('/').pop().split('\\').pop();
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
		try {
			const res = await fetch('/api/implementation');
			const data = await res.json();
			if (data.enabled) {
				renderImplementationStatus(data.report || {});
			}
		} catch (err) {
			console.error('Error loading implementation status:', err);
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
		document.getElementById('edhDecksBody').innerHTML = html || '<tr><td colspan="20">No data</td></tr>';
	}

	function selectDeck(deckName) {
		const deck = allEDHDecksList.find(d => d.deck_name === deckName);
		if (!deck) return;
		
		selectedDeck = deck;
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
				'</tr>';
		}

		document.getElementById('topCardsBody').innerHTML =
			html || '<tr><td colspan="5">No cards for this commander</td></tr>';
	}
		let allEDHDecks = [];

		function renderTopCards(data) {
		allEDHDecks = data.decks || [];
		populateCommanderDropdown(allEDHDecks);
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

			for (let [name, perf] of Object.entries(cards)) {
				currentCardLibraryRows.push({
					name: name,
					casts: perf.casts || 0,
					wins: perf.wins || 0,
					winRate: perf.casts > 0 ? (perf.wins / perf.casts * 100) : 0,
					image_url: perf.image_url || ''
				});
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
					'</tr>';
			}

			document.getElementById('cardLibraryBody').innerHTML =
				html || '<tr><td colspan="4">No matching cards</td></tr>';

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

		function renderStackedBar(label, impl, total) {
			const pct = total > 0 ? (impl / total * 100) : 0;
			const unimpl = total - impl;
			const implPct = pct.toFixed(1);
			const unimplPct = total > 0 ? ((unimpl / total) * 100).toFixed(1) : '0.0';
			let tooltip = label + ': ' + impl + ' implemented out of ' + total + ' cards (' + implPct + '%)';
			return '<div style="margin-bottom:10px;" title="' + tooltip + '">' +
				'<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:4px;">' +
					'<span style="font-size:13px;color:#fff;font-weight:600;">' + label + '</span>' +
					'<span style="font-size:12px;color:#888;">' + impl + '/' + total + ' (' + implPct + '%)</span>' +
				'</div>' +
				'<div style="width:100%;height:18px;background:#141829;border-radius:4px;overflow:hidden;display:flex;">' +
					'<div style="width:' + implPct + '%;height:100%;background:#5a6dd8;display:flex;align-items:center;justify-content:center;font-size:10px;color:#fff;font-weight:bold;white-space:nowrap;overflow:hidden;">' + (impl > 0 ? impl : '') + '</div>' +
					'<div style="width:' + unimplPct + '%;height:100%;background:#e74c3c;display:flex;align-items:center;justify-content:center;font-size:10px;color:#fff;font-weight:bold;white-space:nowrap;overflow:hidden;">' + (unimpl > 0 ? unimpl : '') + '</div>' +
				'</div>' +
				'<div style="display:flex;justify-content:space-between;margin-top:2px;">' +
					'<span style="font-size:10px;color:#5a6dd8;">implemented</span>' +
					'<span style="font-size:10px;color:#e74c3c;">remaining: ' + unimpl + '</span>' +
				'</div>' +
			'</div>';
		}

		function populateCommanderDropdown(decks) {
			const sel = document.getElementById('topCardsCommander');
			if (!sel) return;

			const seen = new Set();

			for (let d of decks) {
				if (d.commander_name && !seen.has(d.commander_name)) {
					seen.add(d.commander_name);
					const opt = document.createElement('option');
					opt.value = d.commander_name;
					opt.textContent = d.commander_name;
					sel.appendChild(opt);
				}
			}
		}

		let implSortKey = 'name', implSortAsc = true;
		let currentImplData = {};
		let currentImplRows = [];
		let implFilterText = '';
		function applyImplFilterAndSort() {
			let rows = (currentImplData.unimplemented_cards || []).slice();
			let filter = implFilterText.trim().toLowerCase();
			if (filter) {
				rows = rows.filter(c => {
					return (c.name || '').toLowerCase().includes(filter) ||
						(c.type || '').toLowerCase().includes(filter) ||
						(c.set || '').toLowerCase().includes(filter) ||
						(c.colors || '').toLowerCase().includes(filter) ||
						(c.reason || '').toLowerCase().includes(filter);
				});
			}
			rows.sort((a, b) => {
				let av = a[implSortKey] || '', bv = b[implSortKey] || '';
				if (av < bv) return implSortAsc ? -1 : 1;
				if (av > bv) return implSortAsc ? 1 : -1;
				return 0;
			});
			currentImplRows = rows;
			renderImplementationTable(rows);
		}
		function sortImplTable(key) {
			if (implSortKey === key) implSortAsc = !implSortAsc;
			else { implSortKey = key; implSortAsc = true; }
			applyImplFilterAndSort();
		}
		function filterImplCards() {
			implFilterText = document.getElementById('implSearch').value || '';
			applyImplFilterAndSort();
		}
		function renderImplementationTable(rows) {
			let html = '';
			for (let c of rows.slice(0, 500)) {
				let tooltip = (c.name || 'Unknown') + ' — ' + (c.type || 'Unknown') + ' — ' + (c.reason || 'No reason given');
				html += '<tr title="' + tooltip + '"><td>' + (c.name || '') + '</td><td>' + (c.type || '') + '</td><td>' + (c.set || '') + '</td><td>' + (c.colors || 'C') + '</td><td>' + (c.reason || '') + '</td></tr>';
			}
			document.getElementById('implCardsBody').innerHTML = html || '<tr><td colspan="5">No unimplemented cards match this search. The card may be fully implemented or not in the database.</td></tr>';
			let total = (currentImplData.unimplemented_cards || []).length;
			let meta = '';
			if (implFilterText.trim()) {
				meta = 'Showing ' + rows.length + ' of ' + total + ' unimplemented cards (search: "' + implFilterText.trim() + '")';
				if (rows.length > 500) meta += '. Display limited to first 500 results.';
			} else {
				meta = 'Showing ' + Math.min(rows.length, 500) + ' of ' + total + ' unimplemented cards';
				if (rows.length > 500) meta += '. Scroll or refine search to see more.';
			}
			document.getElementById('implSearchMeta').textContent = meta;
		}
		function renderImplementationStatus(report) {
			currentImplData = report;
			implFilterText = '';
			let total = report.total_cards || 0;
			let impl = report.implemented_count || 0;
			let unimpl = report.unimplemented_count || 0;
			let pct = report.percentage || 0;
			let summaryHtml = '<div class="card" title="Total cards in the card database"><h2>Total Cards</h2><div class="value">' + total + '</div></div>';
			summaryHtml += '<div class="card" title="Cards fully supported by the ability parser and execution engine"><h2>Implemented</h2><div class="value">' + impl + '</div><span class="unit">' + pct.toFixed(1) + '%</span></div>';
			summaryHtml += '<div class="card" title="Cards the engine cannot yet fully execute"><h2>Unimplemented</h2><div class="value">' + unimpl + '</div><span class="unit">' + (100 - pct).toFixed(1) + '%</span></div>';
			document.getElementById('implSummary').innerHTML = summaryHtml;

			let colorHtml = '';
			for (let b of (report.by_color || [])) {
				colorHtml += renderStackedBar(b.name, b.implemented, b.total);
			}
			document.getElementById('implByColor').innerHTML = colorHtml || '<div class="loading">No data</div>';

			let setHtml = '';
			for (let b of (report.by_set || [])) {
				setHtml += renderStackedBar(b.name, b.implemented, b.total);
			}
			document.getElementById('implBySet').innerHTML = setHtml || '<div class="loading">No data</div>';

			let typeHtml = '';
			for (let b of (report.by_type || [])) {
				typeHtml += renderStackedBar(b.name, b.implemented, b.total);
			}
			document.getElementById('implByType').innerHTML = typeHtml || '<div class="loading">No data</div>';

			applyImplFilterAndSort();
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
				status.textContent = '✓ ' + file.name + ' uploaded';
				status.style.color = '#4ecdc4';
				fileInput.value = '';
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

	setInterval(loadResults, 5000);
	setInterval(updateGameStatus, 2000);
		setInterval(loadEDHGames, 5000);
	loadResults();
		loadEDHGames();
