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
		if (!deckName) { alert('Select a deck in Test Results first'); return; }
		showTab('my-deck');
		const nameInput = document.getElementById('deckName');
		if (nameInput) nameInput.value = getDeckDisplayName(deckName);
		goToMyDeck();
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
	let cardDBMeta = {};
	let myDeckGroupBy = '';
	
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

	function renderUploadedDecks() {
		const container = document.getElementById('uploadedDecksList');
		const section = document.getElementById('uploadedDecksManager');
		if (!container || !section) return;
		const names = Array.from(uploadedDeckNames);
		if (names.length === 0) {
			section.style.display = 'none';
			return;
		}
		section.style.display = '';
		let html = '<div style="display:flex; flex-direction:column; gap:6px;">';
		for (let name of names) {
			const escaped = name.replace(/\\/g, "\\\\").replace(/'/g, "\\'");
			html += '<div style="display:flex; justify-content:space-between; align-items:center; padding:6px; background:#1a1a1a; border-radius:4px;">';
			html += '<span>★ ' + escapeHtml(name) + '</span>';
			html += '<button onclick="deleteUploadedDeck(\'' + escaped + '\')" style="padding:2px 8px;background:#e74c3c;color:#fff;border:none;border-radius:3px;cursor:pointer;font-size:0.75em;" title="Remove uploaded deck">✕</button>';
			html += '</div>';
		}
		html += '</div>';
		container.innerHTML = html;
	}

	async function deleteUploadedDeck(name) {
		try {
			const res = await fetch('/api/uploaded-decks?name=' + encodeURIComponent(name), { method: 'DELETE' });
			if (res.ok) {
				uploadedDeckNames.delete(name);
				saveUploadedDecks();
				renderUploadedDecks();
				filterEDHDecks();
				loadResults();
			}
		} catch (err) {
			console.error('Failed to delete uploaded deck:', err);
		}
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

	// Helper: fetch with an AbortController timeout so a stuck request
	// doesn't block polls forever.  Returns null on error / timeout.
	async function fetchWithTimeout(url, ms) {
		const ctrl = new AbortController();
		const id = setTimeout(() => ctrl.abort(), ms);
		try {
			const res = await fetch(url, { signal: ctrl.signal });
			return res;
		} finally {
			clearTimeout(id);
		}
	}

	async function loadResults() {
		const activeTab = document.querySelector('.tab-content.active');
		const tabId = activeTab ? activeTab.id : '';
		
		const fetches = [];
		if (tabId === 'overview' || tabId === 'card-analysis' || !tabId) {
			const edhURL = '/api/edh-results?light=1' + (document.getElementById('showOnlyUploaded').checked ? '&uploaded=1' : '');
			fetches.push(fetchWithTimeout(edhURL, 10000).catch(() => null));
		} else {
			fetches.push(Promise.resolve(null));
		}
		
		if (tabId === 'card-analysis' || tabId === 'my-deck' || !tabId) {
			fetches.push(fetchWithTimeout('/api/card-library', 10000).catch(() => null));
		} else {
			fetches.push(Promise.resolve(null));
		}
		
		if (!tabId || tabId === 'overview') {
			fetches.push(fetchWithTimeout('/api/results', 10000).catch(() => null));
			fetches.push(fetchWithTimeout('/api/edh-games', 10000).catch(() => null));
		} else {
			fetches.push(Promise.resolve(null));
			fetches.push(Promise.resolve(null));
		}
		
		const [edhRes, libRes, resultsRes, gamesRes] = await Promise.all(fetches);

		if (resultsRes && resultsRes.ok) {
			try {
				const data = await resultsRes.json();
				renderSummary(data);
				renderWinRateChart(data);
				renderTopDecksChart(data);
			} catch (err) {
				console.error('Error parsing results:', err);
			}
		}

		if (edhRes && edhRes.ok) {
			try {
				const data = await edhRes.json();
				if (data.enabled) {
					document.getElementById('edhSummarySection').style.display = '';
					renderEDHSummary(data.summary || {});
					renderEDH(data);
					renderTopCards(data);
				}
			} catch (err) {
				console.error('Error parsing EDH results:', err);
			}
		}

		if (libRes && libRes.ok) {
			try {
				const data = await libRes.json();
				if (data.enabled) {
					renderCardLibrary(data);
				}
			} catch (err) {
				console.error('Error parsing card library:', err);
			}
		}
		
		if (gamesRes && gamesRes.ok) {
			try {
				const data = await gamesRes.json();
				if (data.enabled) renderEDHGames(data.games || []);
			} catch (err) {
				console.error('Error parsing EDH games:', err);
			}
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
			html += '<tr title="' + d.name + ': ' + d.wins + 'W / ' + d.losses + 'L (' + (d.win_rate || 0).toFixed(1) + '% win rate)"><td><span class="deck-name-truncate">' + escapeHtml(d.name) + '</span></td><td>' + d.wins + '</td><td>' + d.losses + '</td><td><strong>' + (d.win_rate || 0).toFixed(1) + '%</strong></td></tr>';
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

		// Filter by search term only (uploaded filter is server-side now)
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
			let isUp = isUploadedDeck(d);
			let tag = isUp ? ' <span style="color:#4ecdc4; font-size:0.75em;">★</span>' : '';
			let safeName = d.deck_name.replace(/\\/g, "\\\\").replace(/'/g, "\\'");
			let delBtn = isUp ? '<button onclick="event.stopPropagation(); deleteUploadedDeck(\'' + safeName + '\')" style="padding:2px 8px;background:#e74c3c;color:#fff;border:none;border-radius:3px;cursor:pointer;font-size:0.75em;" title="Delete uploaded deck">✕</button>' : '';
			html += '<tr title="' + tooltip + '" onclick="selectDeck(' + "'" + escapedDeckName + "'" + ')"' + selected + ' style="cursor:pointer;">'
				+ '<td>' + delBtn + ' <span class="deck-name-truncate">' + escapeHtml(displayName) + '</span>' + tag + '</td>'
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

	async function selectDeck(deckName) {
		const deck = allEDHDecksList.find(d => d.deck_name === deckName);
		if (!deck) return;

		selectedDeck = deck;
		setActiveDeck(deckName);
		document.getElementById('selectedDeckStats').style.display = '';
		document.getElementById('clearDeckSelection').style.display = 'inline-block';

		document.getElementById('selectedDeckName').textContent = getDeckDisplayName(deck.deck_name);
		document.getElementById('selectedDeckCommander').textContent = deck.commander_name || 'Unknown';
		document.getElementById('selectedDeckGames').textContent = deck.games;
		document.getElementById('selectedDeckWinRate').textContent = (deck.win_rate || 0).toFixed(1) + '%';
		document.getElementById('selectedDeckAvgLands').textContent = (deck.avg_lands_played || 0).toFixed(1);
		document.getElementById('selectedDeckAvgCards').textContent = (deck.avg_cards_played || 0).toFixed(1);
		document.getElementById('selectedDeckAvgManaProduced').textContent = (deck.avg_mana_produced || 0).toFixed(1);
		document.getElementById('selectedDeckStorm').textContent = deck.max_storm_count || 0;
		document.getElementById('selCombatWins').textContent = deck.combat_wins || 0;
		document.getElementById('selEffectWins').textContent = deck.effect_wins || 0;
		document.getElementById('selDeckoutWins').textContent = deck.deckout_wins || 0;
		document.getElementById('selAvgManaSpent').textContent = (deck.avg_mana_spent || 0).toFixed(1) + '/turn';
		document.getElementById('selAvgCreatures').textContent = (deck.avg_creatures_cast || 0).toFixed(1) + '/pod';
		document.getElementById('selAvgCombat').textContent = (deck.avg_combat_damage || 0).toFixed(1) + '/pod';

		if (!deck.card_stats || Object.keys(deck.card_stats || {}).length === 0) {
			try {
				const res = await fetch('/api/edh-results?deck=' + encodeURIComponent(deckName));
				const data = await res.json();
				if (data.enabled && data.decks && data.decks.length > 0) {
					selectedDeck.card_stats = data.decks[0].card_stats || {};
				}
			} catch (err) {
				console.error('Error loading deck cards:', err);
			}
		}
		renderSelectedDeckCards();
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
			if (stats.casts >= 3) {
				const meta = cardDBMeta[name] || {};
				cards.push({
					name: name,
					casts: stats.casts,
					wins: stats.wins || 0,
					winRate: stats.casts > 0 ? ((stats.wins || 0) / stats.casts * 100) : 0,
					typeLine: meta.type_line || '',
					cmc: meta.cmc || 0,
					cardType: classifyCardType(meta.type_line || '')
				});
			}
		}
		
		cards.sort((a, b) => {
			let av = a[selectedDeckCardsSortKey];
			let bv = b[selectedDeckCardsSortKey];
			if (typeof av === 'string') av = av.toLowerCase();
			if (typeof bv === 'string') bv = bv.toLowerCase();
			if (av < bv) return selectedDeckCardsSortAsc ? -1 : 1;
			if (av > bv) return selectedDeckCardsSortAsc ? 1 : -1;
			return 0;
		});
		
		const deckWinRate = selectedDeck ? (selectedDeck.win_rate || 0) : 0;
		let currentGroup = '';
		let html = '<div style="margin-bottom:10px; display:flex; gap:8px; flex-wrap:wrap;">';
		html += '<span style="color:#888; font-size:0.85em;">Sort: </span>';
		const sortOpts = [
			{key:'name', label:'Name'}, {key:'winRate', label:'Win Rate'},
			{key:'casts', label:'Casts'}, {key:'wins', label:'Wins'},
			{key:'cardType', label:'Type'}, {key:'cmc', label:'CMC'}
		];
		for (let opt of sortOpts) {
			const active = selectedDeckCardsSortKey === opt.key ? ' style="background:#5a6dd8;color:#fff;"' : ' style="background:#2a2a2a;color:#ccc;"';
			html += '<button onclick="sortSelectedDeckCards(\'' + opt.key + '\')" ' + active + '>' + opt.label + (selectedDeckCardsSortKey === opt.key ? (selectedDeckCardsSortAsc ? ' ↑' : ' ↓') : '') + '</button>';
		}
		html += '<span style="color:#888; font-size:0.85em; margin-left:12px;">Group: </span>';
		const groupOpts = ['', 'cardType', 'cmc'];
		const groupLabels = { '': 'None', 'cardType': 'Type', 'cmc': 'CMC' };
		for (let g of groupOpts) {
			const active = myDeckGroupBy === g ? ' style="background:#e67e22;color:#fff;"' : ' style="background:#2a2a2a;color:#ccc;"';
			html += '<button onclick="setMyDeckGroup(\'' + g + '\')" ' + active + '>' + groupLabels[g] + '</button>';
		}
		html += '</div>';
		html += '<table style="width:100%;border-collapse:collapse;"><thead><tr><th style="text-align:left;padding:6px;border-bottom:1px solid #333;">Card</th><th style="padding:6px;border-bottom:1px solid #333;">Type</th><th style="padding:6px;border-bottom:1px solid #333;">CMC</th><th style="padding:6px;border-bottom:1px solid #333;">Casts</th><th style="padding:6px;border-bottom:1px solid #333;">Wins</th><th style="padding:6px;border-bottom:1px solid #333;">Win Rate</th></tr></thead><tbody>';
		
		for (let c of cards.slice(0, 200)) {
			const winRateDiff = c.winRate - deckWinRate;
			let wrColor;
			if (winRateDiff > 5) wrColor = '#2ecc71';
			else if (winRateDiff < -10) wrColor = '#e74c3c';
			else wrColor = '#f39c12';
			const prevGroup = currentGroup;
			if (myDeckGroupBy === 'cardType') currentGroup = c.cardType || 'Other';
			else if (myDeckGroupBy === 'cmc') currentGroup = 'CMC ' + Math.floor(c.cmc);
			if (myDeckGroupBy && currentGroup !== prevGroup) {
				html += '<tr><td colspan="6" style="padding:6px;background:#1a1f3a;color:#5a6dd8;font-weight:bold;font-size:0.9em;">' + currentGroup + '</td></tr>';
			}
			html += '<tr>'
				+ '<td>' + cardLink(c.name) + '</td>'
				+ '<td style="color:#888;">' + (c.typeLine || '-') + '</td>'
				+ '<td>' + (c.cmc || '-') + '</td>'
				+ '<td>' + c.casts + '</td>'
				+ '<td>' + c.wins + '</td>'
				+ '<td style="color:' + wrColor + ';"><strong>' + c.winRate.toFixed(1) + '%</strong></td>'
				+ '</tr>';
		}
		html += '</tbody></table>';

		document.getElementById('selectedDeckCardsBody').innerHTML = html || '<tr><td colspan="6">No cards with 3+ casts</td></tr>';
	}
	
	function classifyCardType(typeLine) {
		if (!typeLine) return 'Other';
		const t = typeLine.toLowerCase();
		if (t.includes('creature')) return 'Creature';
		if (t.includes('instant') && !t.includes('sorcery')) return 'Instant';
		if (t.includes('sorcery')) return 'Sorcery';
		if (t.includes('artifact') && t.includes('creature')) return 'Artifact Creature';
		if (t.includes('artifact')) return 'Artifact';
		if (t.includes('enchantment') && t.includes('creature')) return 'Enchantment Creature';
		if (t.includes('enchantment')) return 'Enchantment';
		if (t.includes('planeswalker')) return 'Planeswalker';
		if (t.includes('land')) return 'Land';
		return 'Other';
	}
	
	function setMyDeckGroup(group) {
		myDeckGroupBy = group;
		renderSelectedDeckCards();
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
				winRate: (perf.wins / perf.casts) * 100,
				image_url: (globalCardLibraryMap[name.toLowerCase()] || {}).image_url || ''
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
					'<td><span class="deck-name-truncate">' + escapeHtml(c.deck) + '</span></td>' +
					'<td>' + img + cardLink(c.name) + '</td>' +
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
		if (allEDHDecks.length > 0 && allEDHDecks[0].card_stats) {
			renderTopCardsForCommander();
		}
	}
		function populateCommanderDropdown(decks) {
			const select = document.getElementById("commanderSelect");
			if (!select) return;

			const previousValue = select.value;

			const commanders = [...new Set(
				decks.map(d => d.commander_name).filter(Boolean)
			)].sort();

			select.innerHTML = '<option value="">All Commanders</option>' +
				commanders.map(c => '<option value="' + c + '">' + c + '</option>').join("");

			// Restore previous selection if that commander still exists
			if (previousValue && commanders.includes(previousValue)) {
				select.value = previousValue;
			}
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

		let lastCardLibraryHash = '';

		function renderCardLibrary(data) {
			const cards = data.cards || {};
			const newHash = Object.keys(cards).sort().join('|') + '|' +
				Object.values(cards).map(p => (p.casts || 0) + ':' + (p.wins || 0)).join('|');
			if (newHash === lastCardLibraryHash && currentCardLibraryRows.length > 0) {
				return;
			}
			lastCardLibraryHash = newHash;
			currentCardLibraryRows = [];
			globalCardLibraryMap = {};
			cardDBMeta = {};
			const rawMeta = data.card_db || {};
			for (let name of Object.keys(rawMeta)) {
				const m = rawMeta[name];
				const lib = globalCardLibraryMap[name.toLowerCase()];
				if (lib && lib.image_url) m.image_url = lib.image_url;
				cardDBMeta[name.toLowerCase()] = m;
			}

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
				const escapedName = c.name.replace(/\\/g, "\\\\").replace(/'/g, "\\'");

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

		let currentGameLog = null;
		let gameLogSummaries = [];
		let lastGameLogHash = '';

		async function loadGameLogList() {
			try {
				const res = await fetch('/api/game-log-list');
				const data = await res.json();
				if (!data.enabled) return;
				const sums = data.games || [];
				const newHash = sums.map(s => s.id + ':' + (s.winner||'') + ':' + s.event_count).join('|');
				if (newHash === lastGameLogHash) return;
				lastGameLogHash = newHash;
				gameLogSummaries = sums;
				populateGameLogSelector();
			} catch (err) {
				console.error('Error loading game log list:', err);
			}
		}

		function populateGameLogSelector() {
			const sel = document.getElementById('gameLogSelector');
			if (!sel) return;
			const prevVal = sel.value;
			sel.innerHTML = '<option value="">Select a game...</option>';
			for (let i = gameLogSummaries.length - 1; i >= 0; i--) {
				const s = gameLogSummaries[i];
				const players = (s.players || []).join(', ');
				sel.innerHTML += '<option value="' + s.id + '">#' + (i + 1) + ' — ' + (s.winner || 'Draw') + ' — ' + s.turns + ' turns — ' + players + '</option>';
			}
			sel.value = prevVal;
		}

		async function loadGameLog() {
			const sel = document.getElementById('gameLogSelector');
			if (!sel || !sel.value) {
				document.getElementById('gameLogSummary').style.display = 'none';
				document.getElementById('gameLogContent').innerHTML = '<div class="loading">Select a game to view its event log</div>';
				return;
			}
			const id = parseInt(sel.value);
			document.getElementById('gameLogContent').innerHTML = '<div class="loading">Loading...</div>';
			try {
				const res = await fetch('/api/game-log?id=' + id);
				const data = await res.json();
				if (!data.enabled || !data.game) {
					document.getElementById('gameLogContent').innerHTML = '<div class="error">Game not found</div>';
					return;
				}
				currentGameLog = data.game;
				const g = currentGameLog;
				document.getElementById('gameLogSummary').style.display = '';
				document.getElementById('glWinner').textContent = g.Winner || 'Draw';
				document.getElementById('glTurns').textContent = g.Turns;
				document.getElementById('glPlayers').textContent = (g.Players || []).map(p => p.DeckName).join(', ');
				document.getElementById('glMana').textContent = g.TotalManaSpent || 0;
				document.getElementById('glCards').textContent = g.TotalCardsPlayed || 0;
				document.getElementById('glCombat').textContent = g.TotalCombatDamage || 0;
				renderGameLogEvents();
			} catch (err) {
				console.error('Error loading game log:', err);
				document.getElementById('gameLogContent').innerHTML = '<div class="error">Error loading game log</div>';
			}
		}

		function renderGameLogEvents() {
			if (!currentGameLog) return;
			const events = currentGameLog.Events || [];
			const filter = document.getElementById('gameLogEventFilter').value;
			const showAll = document.getElementById('gameLogShowAllEvents').checked;

			let filtered = events;
			if (filter) {
				filtered = events.filter(e => e.kind === filter);
			}
			if (!showAll) {
				const importantKinds = ['game_start', 'turn_start', 'commander_cast', 'fetch_activated', 'spell_countered', 'attack_declared', 'combat_resolved', 'player_eliminated', 'game_end'];
				filtered = filtered.filter(e => importantKinds.includes(e.kind));
			}

			const kindColors = {
				'game_start': '#8e44ad',
				'turn_start': '#3498db',
				'land_play': '#2ecc71',
				'creature_summon': '#27ae60',
				'permanent_cast': '#f39c12',
				'commander_cast': '#e67e22',
				'fetch_activated': '#1abc9c',
				'spell_countered': '#e74c3c',
				'attack_declared': '#e74c3c',
				'combat_resolved': '#c0392b',
				'player_eliminated': '#e74c3c',
				'game_end': '#9b59b6'
			};

			let html = '<table style="width:100%;border-collapse:collapse;font-size:0.9em;">';
			html += '<thead><tr><th style="text-align:left;padding:6px;border-bottom:1px solid #333;width:50px;">Turn</th><th style="text-align:left;padding:6px;border-bottom:1px solid #333;width:80px;">Phase</th><th style="text-align:left;padding:6px;border-bottom:1px solid #333;width:140px;">Event</th><th style="text-align:left;padding:6px;border-bottom:1px solid #333;width:120px;">Actor</th><th style="text-align:left;padding:6px;border-bottom:1px solid #333;width:120px;">Target</th><th style="text-align:left;padding:6px;border-bottom:1px solid #333;">Detail</th></tr></thead><tbody>';

			for (let e of filtered) {
				const color = kindColors[e.kind] || '#888';
				const kindLabel = e.kind.replace(/_/g, ' ');
				html += '<tr style="border-bottom:1px solid #1a1f3a;">';
				html += '<td style="padding:4px 6px;">' + e.turn + '</td>';
				html += '<td style="padding:4px 6px;color:#888;">' + (e.phase || '-') + '</td>';
				html += '<td style="padding:4px 6px;color:' + color + ';font-weight:bold;">' + kindLabel + '</td>';
				html += '<td style="padding:4px 6px;">' + (e.actor || '-') + '</td>';
				html += '<td style="padding:4px 6px;">' + (e.target || '-') + '</td>';
				html += '<td style="padding:4px 6px;color:#aaa;max-width:300px;word-break:break-word;">' + (e.detail || '') + '</td>';
				html += '</tr>';
			}

			html += '</tbody></table>';

			if (filtered.length === 0) {
				html = '<div style="color:#888; padding:20px; text-align:center;">No events match the current filter</div>';
			} else {
				html += '<div style="color:#888; padding:8px; font-size:0.85em;">Showing ' + filtered.length + ' of ' + events.length + ' events</div>';
			}

			document.getElementById('gameLogContent').innerHTML = html;
		}

	// Chart.js instances
	let winRateChart = null;
	let topDecksChart = null;

	function renderWinRateChart(data) {
		let ranges = {
			'0-25%': 0,
			'25-50%': 0,
			'50-75%': 0,
			'75-100%': 0
		};

		// Use server-computed histogram when available (avoids iterating
		// thousands of deck entries shipped only for this chart).
		if (data.win_rate_buckets) {
			for (let b of data.win_rate_buckets) {
				ranges[b.label] = b.count;
			}
		} else {
			// Fallback for older server versions.
			const decks = data.decks || [];
			for (let d of decks) {
				const wr = d.win_rate || 0;
				if (wr < 25) ranges['0-25%']++;
				else if (wr < 50) ranges['25-50%']++;
				else if (wr < 75) ranges['50-75%']++;
				else ranges['75-100%']++;
			}
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
		
		decks.sort((a, b) => (b.win_rate || 0) - (a.win_rate || 0));
		const topDecks = decks.slice(0, 8);
		
		const labels = topDecks.map(d => d.name.replace(/\.deck$/, '').substring(0, 18));
		const winRates = topDecks.map(d => (d.win_rate || 0).toFixed(1));
		
		const ctx = document.getElementById('topDecksChart');
		if (!ctx) return;
		
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
				onClick: function(evt, elements) {
					if (elements.length > 0) {
						const idx = elements[0].index;
						const deckName = topDecks[idx].name;
						populateMyDeckFromResults(deckName);
					}
				},
				plugins: {
					legend: { labels: { color: '#ccc' } },
					tooltip: {
						callbacks: {
							label: function(ctx) { return 'WR ' + ctx.raw + '% — click to open in My Deck'; }
						}
					}
				},
				scales: {
					x: { max: 100, ticks: { color: '#888' }, grid: { color: '#333' } },
					y: { ticks: { color: '#888' }, grid: { display: false } }
				}
			}
		});
	}
	
	async function populateMyDeckFromResults(deckName) {
		const deck = allEDHDecksList.find(d => d.deck_name === deckName);
		if (!deck) return;
		
		setActiveDeck(deckName);
		let cardList = [];
		if (deck.card_stats && Object.keys(deck.card_stats).length > 0) {
			cardList = Object.keys(deck.card_stats).map(name => '1 ' + name);
		} else {
			try {
				const res = await fetch('/api/edh-results?deck=' + encodeURIComponent(deckName));
				const data = await res.json();
				if (data.enabled && data.decks && data.decks.length > 0 && data.decks[0].card_stats) {
					cardList = Object.keys(data.decks[0].card_stats).map(name => '1 ' + name);
				}
			} catch (err) { console.error('Error loading deck cards:', err); }
		}
		
		const nameInput = document.getElementById('deckName');
		const listInput = document.getElementById('deckList');
		if (nameInput) nameInput.value = getDeckDisplayName(deckName);
		if (listInput) listInput.value = cardList.join('\n');
		addCardToDeck();
		saveDeckEditorState();
		showTab('my-deck');
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
				btn.textContent = '🔄 Running...';
				pollWhileRunning();
			} else {
				btn.textContent = '▶ Run Games';
				btn.disabled = false;
				const errorMsg = data.error || 'Unknown error';
				alert('Error starting games: ' + errorMsg);
			}
		} catch (err) {
			console.error('Error running games:', err);
			btn.textContent = '▶ Run Games';
			btn.disabled = false;
		}
	}
	
	async function pollWhileRunning() {
		const res = await fetch('/api/game-status');
		const data = await res.json();
		const indicator = document.getElementById('gameStatusIndicator');
		const btn = document.getElementById('runGamesBtn');
		if (data.running) {
			indicator.textContent = '🔄 Games running...';
			indicator.style.color = '#4ecdc4';
			setTimeout(pollWhileRunning, 2000);
		} else {
			indicator.textContent = '✓ Ready';
			indicator.style.color = '#888';
			btn.disabled = false;
			btn.textContent = '▶ Run Games';
			loadResults();
			loadEDHGames();
		}
	}

	async function resetCardLibrary() {
		const status = document.getElementById('resetStatus');
		status.textContent = 'Resetting...';
		status.style.color = '#e74c3c';
		try {
			const res = await fetch('/api/reset-card-library', { method: 'POST' });
			const data = await res.json();
			if (res.ok) {
				status.textContent = '✓ Card library reset';
				status.style.color = '#2ecc71';
				loadResults();
			} else {
				status.textContent = '✗ ' + (data.error || 'Error');
				status.style.color = '#e74c3c';
			}
		} catch (err) {
			status.textContent = '✗ Request failed';
			status.style.color = '#e74c3c';
		}
		setTimeout(() => { status.textContent = ''; }, 5000);
	}

	async function resetGameLogs() {
		const status = document.getElementById('resetStatus');
		status.textContent = 'Resetting...';
		status.style.color = '#e67e22';
		try {
			const res = await fetch('/api/reset-game-logs', { method: 'POST' });
			const data = await res.json();
			if (res.ok) {
				status.textContent = '✓ Game logs cleared';
				status.style.color = '#2ecc71';
				loadResults();
			} else {
				status.textContent = '✗ ' + (data.error || 'Error');
				status.style.color = '#e74c3c';
			}
		} catch (err) {
			status.textContent = '✗ Request failed';
			status.style.color = '#e74c3c';
		}
		setTimeout(() => { status.textContent = ''; }, 5000);
	}

	let previousRunning = false;

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
				if (!previousRunning) pollWhileRunning();
			} else {
				indicator.textContent = '✓ Ready';
				indicator.style.color = '#888';
				btn.disabled = false;
				btn.textContent = '▶ Run Games';
			}

			if (previousRunning && !data.running) {
				console.log('[Polling] Run completed – refreshing results');
				loadResults();
				loadEDHGames();
				loadMatchupMatrix();
			}
			previousRunning = data.running;
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
				renderUploadedDecks();

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
				renderUploadedDecks();
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

			// Build a set of cards already known to be in this deck so we don't suggest them again
			const excludedCards = new Set();
			if (selectedDeck && selectedDeck.card_stats) {
				for (let name of Object.keys(selectedDeck.card_stats)) {
					excludedCards.add(name.toLowerCase());
				}
			}
			if (recs.remove_candidates) {
				for (let c of recs.remove_candidates) {
					excludedCards.add(c.card_name.toLowerCase());
				}
			}
			for (let card of currentDeckList) {
				if (card && card.name) {
					excludedCards.add(card.name.toLowerCase());
				}
			}

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
					const escaped = c.card_name.replace(/\\/g, "\\\\").replace(/'/g, "\\'");
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
				let shown = 0;
				for (let c of recs.add_candidates) {
					if (excludedCards.has(c.card_name.toLowerCase())) continue;
					if (shown >= 10) break;
					shown++;
					const escaped = c.card_name.replace(/\\/g, "\\\\").replace(/'/g, "\\'");
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

	// Populate recommendations deck select - show all available EDH decks,
	// deduplicated by display name. Uploaded decks are highlighted with a star.
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

		// Deduplicate by display name, preferring uploaded decks when a name collides.
		const seen = new Map();
		for (let d of allEDHDecksList) {
			const displayName = getDeckDisplayName(d.deck_name);
			if (seen.has(displayName)) {
				if (isUploadedDeck(d)) {
					seen.set(displayName, d);
				}
				continue;
			}
			seen.set(displayName, d);
		}

		let allDecks = Array.from(seen.values()).sort((a, b) =>
			(getDeckDisplayName(a.deck_name) || '').localeCompare(getDeckDisplayName(b.deck_name) || '')
		);

		// Add all decks to dropdown (display by filename only)
		for (let d of allDecks) {
			const opt = document.createElement('option');
			opt.value = d.deck_name;
			const isUploaded = isUploadedDeck(d);
			opt.textContent = (isUploaded ? '★ ' : '') + getDeckDisplayName(d.deck_name);
			if (isUploaded) {
				opt.style.color = '#4ecdc4';
				opt.style.fontWeight = 'bold';
			}
			select.appendChild(opt);
		}

		// Restore the user's selection if that option still exists,
		// otherwise fall back to the active deck
		if (previousValue && Array.from(select.options).some(o => o.value === previousValue)) {
			select.value = previousValue;
		} else if (activeDeckName && Array.from(select.options).some(o => o.value === activeDeckName)) {
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

		let winRateStr = '-';
		const deckNameInput = document.getElementById('deckName').value.trim();
		const lookupName = activeDeckName || deckNameInput;
		if (lookupName) {
			const deck = allEDHDecksList.find(d =>
				d.deck_name === lookupName ||
				getDeckDisplayName(d.deck_name) === lookupName ||
				getDeckDisplayName(d.deck_name) === deckNameInput
			);
			if (deck && deck.win_rate !== undefined) {
				winRateStr = (deck.win_rate || 0).toFixed(1) + '%';
			}
		}
		const wrEl = document.getElementById('myDeckWinRate');
		if (wrEl) wrEl.textContent = winRateStr;

		updateDeckEditorStats();
		loadInlineRecommendations(lookupName);
	}
	
	async function loadInlineRecommendations(deckName) {
		const container = document.getElementById('inlineRecContent');
		if (!container || !deckName) return;
		try {
			const res = await fetch('/api/card-recommendations?deck=' + encodeURIComponent(deckName));
			const data = await res.json();
			if (!data.enabled || !data.recommendations) {
				container.innerHTML = '<div style="color:#666;">No recommendation data available</div>';
				return;
			}
			const recs = data.recommendations;
			let html = '';
			if (recs.remove_candidates && recs.remove_candidates.length > 0) {
				html += '<div style="margin-bottom:6px;"><strong style="color:#e74c3c;">🗑️ Cut:</strong></div>';
				for (let c of recs.remove_candidates.slice(0, 5)) {
				const escaped = c.card_name.replace(/\\/g, "\\\\").replace(/'/g, "\\'");
				html += '<div style="padding:2px 0; font-size:0.85em;">' + cardLink(c.card_name) + ' <span style="color:#e74c3c;">' + (c.win_rate||0).toFixed(1) + '%</span> (' + (c.casts||0) + ' casts)</div>';
			}
		}
		if (recs.add_candidates && recs.add_candidates.length > 0) {
			html += '<div style="margin-top:6px;"><strong style="color:#2ecc71;">✅ Add:</strong></div>';
			for (let c of recs.add_candidates) {
				// Check if already in deck
				if (deckCards.has(c.card_name.toLowerCase())) continue;
				if (shown >= 5) break;
				shown++;
				const escaped = c.card_name.replace(/\\/g, "\\\\").replace(/'/g, "\\'");
					html += '<div style="padding:2px 0; font-size:0.85em;">' + cardLink(c.card_name) + ' <span style="color:#2ecc71;">' + (c.win_rate||0).toFixed(1) + '%</span> (' + (c.casts||0) + ' global)';
					html += ' <button onclick="addCardToMyDeck(\'' + escaped + '\')" style="padding:1px 6px;background:#4ecdc4;color:#000;border:none;border-radius:2px;cursor:pointer;font-size:0.75em;">+Add</button></div>';
				}
			}
			container.innerHTML = html || '<div style="color:#666;">No recommendations — run more games first</div>';
		} catch (err) {
			container.innerHTML = '<div style="color:#666;">Loading...</div>';
		}
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

	let myDeckEditorSortKey = 'winRate';
	let myDeckEditorSortAsc = false;

	function updateDeckEditorStats() {
		const container = document.getElementById('deckEditorCardStats');
		if (!container || currentDeckList.length === 0) {
			if (container) container.innerHTML = '';
			return;
		}
		let items = [];
		for (let card of currentDeckList) {
			const lib = globalCardLibraryMap[card.name.toLowerCase()];
			const meta = cardDBMeta[card.name.toLowerCase()] || {};
			const wr = lib ? lib.winRate : 0;
			const casts = lib ? lib.casts : 0;
			items.push({
				name: card.name,
				count: card.count,
				winRate: wr,
				casts: casts,
				hasData: !!lib,
				typeLine: meta.type_line || '',
				cmc: meta.cmc || 0,
				cardType: classifyCardType(meta.type_line || '')
			});
		}
		items.sort((a, b) => {
			let av = a[myDeckEditorSortKey];
			let bv = b[myDeckEditorSortKey];
			if (typeof av === 'string') av = av.toLowerCase();
			if (typeof bv === 'string') bv = bv.toLowerCase();
			if (typeof av === 'number' && typeof bv === 'number') return myDeckEditorSortAsc ? av - bv : bv - av;
			if (av < bv) return myDeckEditorSortAsc ? -1 : 1;
			if (av > bv) return myDeckEditorSortAsc ? 1 : -1;
			return 0;
		});
		let html = '<div style="display:flex; gap:8px; align-items:center; margin-bottom:8px; flex-wrap:wrap;">';
		html += '<span style="color:#888; font-size:0.85em;">Sort:</span>';
		const sortOpts = [
			{key:'name', label:'Name'}, {key:'winRate', label:'Win Rate'},
			{key:'casts', label:'Casts'}, {key:'cardType', label:'Type'}, {key:'cmc', label:'CMC'}
		];
		for (let opt of sortOpts) {
			const active = myDeckEditorSortKey === opt.key ? ' style="background:#5a6dd8;color:#fff;"' : ' style="background:#2a2a2a;color:#ccc;"';
			html += '<button onclick="sortMyDeckEditor(\'' + opt.key + '\')" ' + active + '>' + opt.label + (myDeckEditorSortKey === opt.key ? (myDeckEditorSortAsc ? ' ↑' : ' ↓') : '') + '</button>';
		}
		html += '</div>';
		let found = 0;
		for (let item of items) {
			const implWarning = isCardImplemented(item.name) ? '' : ' <span style="color:#e74c3c; font-weight:bold;" title="This card is not fully implemented in the simulator">⚠️</span>';
			const typeInfo = item.typeLine ? ' <span style="color:#888;font-size:0.8em;">(' + item.typeLine + ')</span>' : '';
			if (item.hasData) {
				found++;
				let deckWinRate = 50;
				if (activeDeckName) {
					const deck = allEDHDecksList.find(d => d.deck_name === activeDeckName);
					if (deck) deckWinRate = deck.win_rate || 50;
				}
				const wrColor = item.winRate >= deckWinRate + 5 ? '#2ecc71' : item.winRate >= deckWinRate - 10 ? '#f39c12' : '#e74c3c';
				html += '<div style="display:flex; justify-content:space-between; padding:4px 0; border-bottom:1px solid #1a1f3a;">';
				html += '<span>' + item.count + 'x ' + cardLink(item.name) + implWarning + typeInfo + '</span>';
				html += '<span style="color:' + wrColor + ';">' + item.winRate.toFixed(1) + '% WR (' + item.casts + ' casts) CMC ' + item.cmc + '</span>';
				html += '</div>';
			} else {
				html += '<div style="display:flex; justify-content:space-between; padding:4px 0; border-bottom:1px solid #1a1f3a;">';
				html += '<span>' + item.count + 'x ' + cardLink(item.name) + implWarning + typeInfo + '</span>';
				html += '<span style="color:#666;">no data</span>';
				html += '</div>';
			}
		}
		if (found === 0 && currentDeckList.length > 0) {
			html += '<div style="color:#666; font-size:0.9em;">No performance data for these cards yet. Run more games or search in Card Analysis.</div>';
		}
		container.innerHTML = html;
	}
	
	function sortMyDeckEditor(key) {
		if (myDeckEditorSortKey === key) {
			myDeckEditorSortAsc = !myDeckEditorSortAsc;
		} else {
			myDeckEditorSortKey = key;
			myDeckEditorSortAsc = false;
		}
		updateDeckEditorStats();
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
				renderUploadedDecks();
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
	function escapeHtml(value) {
		return String(value)
			.replace(/&/g, '&amp;')
			.replace(/</g, '&lt;')
			.replace(/>/g, '&gt;')
			.replace(/"/g, '&quot;')
			.replace(/'/g, '&#39;');
	}

	function showCardModal(cardName) {
		const meta = cardDBMeta[cardName.toLowerCase()] || {};
		const lib = globalCardLibraryMap[cardName.toLowerCase()];
		const deckWR = selectedDeck ? (selectedDeck.win_rate || 50) : 50;
		const imgUrl = lib ? lib.image_url : '';
		
		let html = '';
		if (imgUrl) {
			html += '<div style="text-align:center; margin-bottom:12px;"><img src="' + imgUrl + '" style="max-width:100%; max-height:300px; border-radius:8px;"></div>';
		}
		html += '<h3 style="margin:0 0 12px; color:#fff;">' + escapeHtml(cardName) + '</h3>';
		if (meta.type_line) {
			html += '<div style="color:#888; margin-bottom:8px;">' + escapeHtml(meta.type_line);
			if (meta.mana_cost) html += ' — ' + escapeHtml(meta.mana_cost);
			html += '</div>';
		}
		if (meta.cmc !== undefined && meta.cmc > 0) {
			html += '<div style="margin-bottom:4px;">CMC: <strong>' + meta.cmc + '</strong></div>';
		}
		if (meta.colors && meta.colors.length > 0) {
			html += '<div style="margin-bottom:4px;">Colors: <strong>' + meta.colors.join(', ') + '</strong></div>';
		}
		html += '<hr style="border-color:#333; margin:10px 0;">';
		if (lib) {
			const wrColor = lib.winRate >= deckWR + 5 ? '#2ecc71' : lib.winRate >= deckWR - 10 ? '#f39c12' : '#e74c3c';
			html += '<div style="margin-bottom:4px;">Global Casts: <strong>' + lib.casts + '</strong></div>';
			html += '<div style="margin-bottom:4px;">Global Wins: <strong>' + lib.wins + '</strong></div>';
			html += '<div style="margin-bottom:4px; color:' + wrColor + ';">Win Rate: <strong>' + lib.winRate.toFixed(1) + '%</strong></div>';
			if (lib.winRate < deckWR) {
				html += '<div style="color:#e74c3c; font-size:0.85em;">' + (deckWR - lib.winRate).toFixed(1) + '% below deck avg</div>';
			} else if (lib.winRate > deckWR) {
				html += '<div style="color:#2ecc71; font-size:0.85em;">' + (lib.winRate - deckWR).toFixed(1) + '% above deck avg</div>';
			}
		} else {
			html += '<div style="color:#666;">No performance data yet</div>';
		}
		html += '<hr style="border-color:#333; margin:10px 0;">';
		html += '<button onclick="addCardToMyDeck(\'' + cardName.replace(/\\/g, "\\\\").replace(/'/g, "\\'") + '\')" style="padding:6px 14px; background:#4ecdc4; color:#000; border:none; border-radius:4px; cursor:pointer; font-weight:bold;">+ Add to My Deck</button>';
		
		document.getElementById('cardModalContent').innerHTML = html;
		document.getElementById('cardModal').style.display = 'flex';
	}

	function closeCardModal() {
		document.getElementById('cardModal').style.display = 'none';
	}
	
	function cardLink(name) {
		const escaped = escapeHtml(name);
		const safe = name.replace(/\\/g, "\\\\").replace(/'/g, "\\'").replace(/"/g, '&quot;');
		const meta = cardDBMeta[name.toLowerCase()] || {};
		const imgUrl = meta.image_url || '';
		const link = '<a href="javascript:void(0)" onclick="showCardModal(\'' + safe + '\')" style="color:#4ecdc4; cursor:pointer; text-decoration:none;" title="Click for details">' + escaped + '</a>';
		if (imgUrl) {
			return '<span class="card-thumb-wrapper" style="display:inline-flex;align-items:center;gap:6px;cursor:pointer;">' +
				link +
				'<div class="card-preview" style="left:0;top:24px;">' +
					'<img src="' + imgUrl + '" class="card-preview-img">' +
				'</div>' +
			'</span>';
		}
		return link;
	}

	document.addEventListener('DOMContentLoaded', function() {
		const modal = document.getElementById('cardModal');
		if (modal) {
			modal.addEventListener('click', function(e) {
				if (e.target === modal) closeCardModal();
			});
		}
	});

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
				const escapedName = card.name.replace(/\\/g, "\\\\").replace(/'/g, "\\'");

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
				html = '<div class="error" style="grid-column:1/-1;">No cards found matching "' + escapeHtml(query) + '"</div>';
			}

			resultsDiv.innerHTML = html;
		} catch (err) {
			console.error('Error searching cards:', err);
			resultsDiv.innerHTML = '<div class="error">Error searching cards</div>';
		}
	}

	// Matchup Matrix
	let matchupMatrixData = [];
	let lastMatchupHash = '';

	async function loadMatchupMatrix() {
		const bodyEl = document.getElementById('matchupBody');
		if (!bodyEl) return;

		// Only show loading spinner on first load; subsequent refreshes keep existing DOM
		const hadData = matchupMatrixData.length > 0;
		if (!hadData) {
			bodyEl.innerHTML = '<tr><td colspan="7" class="loading">Loading...</td></tr>';
		}

		try {
			const res = await fetch('/api/matchup-matrix');
			const data = await res.json();

			if (!data.enabled) {
				if (!hadData) {
					bodyEl.innerHTML = '<tr><td colspan="7" class="error">Matchup data not available</td></tr>';
				}
				return;
			}

			const newDecks = data.decks || [];
			const newHash = newDecks.map(d =>
				(d.name || '') + ':' + (d.games || 0) + ':' + ((d.win_rate || 0).toFixed(2))
			).join('|');
			if (newHash === lastMatchupHash && newDecks.length > 0) {
				return; // Data unchanged, preserve DOM and scroll state
			}
			lastMatchupHash = newHash;
			matchupMatrixData = newDecks;

			// Preserve scroll position across re-render
			const scrollParent = bodyEl.closest('.table') || bodyEl.parentElement;
			const scrollTop = scrollParent ? scrollParent.scrollTop : 0;

			renderMatchupMatrix(matchupMatrixData);
			// Re-apply any active user filter so it isn't lost on refresh
			filterMatchupMatrix();

			if (scrollParent) scrollParent.scrollTop = scrollTop;
		} catch (err) {
			console.error('Error loading matchups:', err);
			if (!hadData) {
				bodyEl.innerHTML = '<tr><td colspan="7" class="error">Error loading matchup data</td></tr>';
			}
		}
	}

	function renderMatchupMatrix(decks) {
		const bodyEl = document.getElementById('matchupBody');
		if (!bodyEl) return;
		let html = '';

		for (let d of decks) {
			const wrColor = d.win_rate >= 55 ? '#2ecc71' : d.win_rate >= 45 ? '#f39c12' : '#e74c3c';
			const deckNameAttr = JSON.stringify(String(d.name || '')).slice(1, -1).replace(/"/g, '&quot;');
			html += '<tr>';
			html += '<td><span class="deck-name-truncate">' + escapeHtml(getDeckDisplayName(d.name)) + '</span></td>';
			html += '<td>' + d.commander + '</td>';
			html += '<td style="color:' + wrColor + '; font-weight:bold;">' + (d.win_rate || 0).toFixed(1) + '%</td>';
			html += '<td>' + d.games + '</td>';
			html += '<td>' + Math.round(d.games * (d.win_rate || 0) / 100) + '</td>';
			html += '<td>' + Math.round(d.games * (1 - (d.win_rate || 0) / 100)) + '</td>';
			html += '<td><button class="matchup-recs-btn" data-deck="' + deckNameAttr + '" style="padding:4px 10px; background:#5a6dd8; color:#fff; border:none; border-radius:4px; cursor:pointer; font-size:0.8em; font-weight:bold;">🔮 Recs</button></td>';
			html += '</tr>';
		}

		bodyEl.innerHTML = html || '<tr><td colspan="7">No matchup data available</td></tr>';
		// Use event delegation via onclick to avoid accumulating duplicate listeners
		bodyEl.onclick = function(e) {
			const btn = e.target.closest('.matchup-recs-btn');
			if (!btn) return;
			selectDeck(btn.dataset.deck || '');
			goToRecommendations();
		};
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

	let lastSnapshotsHash = '';
	let selectedSnapshotName = '';

	async function loadSnapshotsList() {
		const listEl = document.getElementById('snapshotsList');
		if (!listEl) return;

		// Preserve current selection before potential rebuild
		const prevSelected = listEl.querySelector('tr.selected');
		if (prevSelected) selectedSnapshotName = prevSelected.dataset.name || '';

		try {
			const res = await fetch('/api/snapshots');
			const data = await res.json();

			if (!data.enabled) {
				listEl.innerHTML = '<div class="error">Snapshots not available</div>';
				return;
			}

			const snapshots = data.snapshots || [];
			const newHash = snapshots.map(s => s.name + ':' + s.timestamp).join('|');
			if (newHash === lastSnapshotsHash) {
				return; // Data unchanged, preserve DOM and selection
			}
			lastSnapshotsHash = newHash;
			let html = '';

			if (snapshots.length === 0) {
				html = '<div style="color:#888; padding:15px;">No snapshots saved yet. Save one to get started!</div>';
			} else {
				html = '<table class="table" style="font-size:0.95em;"><thead><tr><th>Name</th><th>Date</th><th>Decks</th><th>Avg WR</th></tr></thead><tbody>';
				for (let snap of snapshots) {
					const date = new Date(snap.timestamp).toLocaleString();
					const selClass = (snap.name === selectedSnapshotName) ? ' class="selected"' : '';
					html += '<tr data-name="' + escapeHtml(snap.name) + '"' + selClass + '>';
					html += '<td>' + escapeHtml(snap.name) + '</td>';
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
					const escaped = JSON.stringify(cardName);
					html += '<button onclick="addCardToMyDeck(' + escaped + ')" style="padding:4px 10px; background:#4ecdc4; color:#000; border:none; border-radius:4px; cursor:pointer; font-size:0.8em; font-weight:bold;">+ Add</button>';
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
	renderUploadedDecks();
	loadDeckEditorState();
	loadImplementationStatus();
	setupRecDeckSelectHandler();

	// ===== Polling =====
	//
	// Each endpoint gets its own interval so a slow endpoint doesn't
	// block faster, more important polls.  Start times are staggered
	// so not all requests fire in the same event-loop tick.

	const POLL = {
		results:     { fn: loadResults,            ms: 5000, delay: 500 },
		edhGames:    { fn: loadEDHGames,           ms: 8000, delay: 1500 },
		gameLogList: { fn: loadGameLogList,        ms: 8000, delay: 2000 },
		matchupMatrix: { fn: loadMatchupMatrix,    ms: 10000, delay: 2500 },
		snapshots:   { fn: loadSnapshotsList,      ms: 15000, delay: 3500 },
	};

	let pollingTimers = {};

	function startPolling() {
		for (const [key, cfg] of Object.entries(POLL)) {
			if (pollingTimers[key]) continue;
			// Stagger the initial fire so requests don't pile up.
			setTimeout(() => {
				cfg.fn();
				pollingTimers[key] = setInterval(cfg.fn, cfg.ms);
				console.log('[Polling] Started ' + key + ' every ' + cfg.ms + 'ms');
			}, cfg.delay);
		}
	}

	function stopPolling() {
		for (const key in pollingTimers) {
			if (pollingTimers[key]) {
				clearInterval(pollingTimers[key]);
				pollingTimers[key] = null;
				console.log('[Polling] Stopped ' + key);
			}
		}
	}

	// Use Page Visibility API to pause/resume polling
	document.addEventListener('visibilitychange', () => {
		if (document.hidden) {
			stopPolling();
		} else {
			startPolling();
		}
	});

	// Initial loads (fire immediately, not staggered)
	loadSnapshotsList();
	loadResults();
	loadEDHGames();
	loadGameLogList();
	loadMatchupMatrix();
	updateGameStatus();
	startPolling();
	
	// Check DB connectivity
	fetch('/api/db-status').then(r => r.json()).then(d => {
		const indicator = document.getElementById('gameStatusIndicator');
		if (d.connected && d.total_pods > 0) {
			indicator.textContent = '💾 DB: ' + d.total_pods + ' pods, ' + (d.decks||0) + ' decks, ' + (d.total_cards_tracked||0) + ' cards';
			indicator.style.color = '#2ecc71';
		} else if (d.connected) {
			indicator.textContent = '💾 DB connected (no data yet)';
			indicator.style.color = '#f39c12';
		} else {
			indicator.textContent = '⚠️ No database — results will not persist';
			indicator.style.color = '#e74c3c';
		}
	});

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
