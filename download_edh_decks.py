#!/usr/bin/env python3
"""
Download decks from TopDeck.gg EDH tournaments

This script fetches recent EDH tournament data from TopDeck.gg and saves all
decklists to local files for further analysis or simulation.

SETUP:
  1. Get a free API key from https://topdeck.gg/account
  2. Set the TOPDECK_API_KEY environment variable:
     export TOPDECK_API_KEY="your_api_key_here"
  3. Run the script:
     python3 download_edh_decks.py

EXAMPLES:
  # Download decks from last 30 days (default)
  python3 download_edh_decks.py

  # Download from last 60 days, minimum 30 participants
  python3 download_edh_decks.py --days 60 --min-participants 30

  # Save to custom directory
  python3 download_edh_decks.py --output custom_decks/

  # Show what would be downloaded without saving
  python3 download_edh_decks.py --dry-run

  # Download and skip existing files silently
  python3 download_edh_decks.py --quiet
"""
import json
import requests
import time
import os
import re
import sys
import argparse
from datetime import datetime, timedelta
from pathlib import Path

BASE_URL = "https://topdeck.gg/api/v2"
RATE_LIMIT_DELAY = 0.6  # seconds between requests (100/min rate limit)

# Get API key from environment or prompt
API_KEY = os.environ.get('TOPDECK_API_KEY')
if not API_KEY:
    print("ERROR: TOPDECK_API_KEY environment variable not set")
    print("Get a free API key from: https://topdeck.gg/account")
    print("Then set it: export TOPDECK_API_KEY='your_key_here'")
    sys.exit(1)

HEADERS = {
    'Authorization': API_KEY,
    'Content-Type': 'application/json'
}

def get_edh_tournaments(days_back=30, participant_min=20):
    """Fetch recent EDH tournaments from TopDeck.gg"""
    print(f"Fetching EDH tournaments from the last {days_back} days...")
    print(f"  Minimum participants: {participant_min}")
    
    data = {
        'game': 'Magic: The Gathering',
        'format': 'EDH',
        'last': days_back,
        'participantMin': participant_min,
        'columns': ['name', 'decklist', 'wins', 'draws', 'losses']
    }
    
    time.sleep(RATE_LIMIT_DELAY)
    response = requests.post(f"{BASE_URL}/tournaments", json=data, headers=HEADERS)
    
    if response.status_code == 401:
        print("ERROR: Invalid API key. Get one from https://topdeck.gg/account")
        sys.exit(1)
    
    response.raise_for_status()
    
    tournaments = response.json()
    print(f"Found {len(tournaments)} tournaments")
    
    return tournaments

def extract_decks_from_tournaments(tournaments):
    """Extract all decklists from tournament standings"""

    decks = []

    for tournament in tournaments:
        tournament_name = tournament.get('tournamentName', 'Unknown')

        standings = tournament.get('standings', [])

        for player in standings:

            #
            # IMPORTANT:
            # Prefer structured deckObj over raw decklist text
            #

            deck_obj = player.get('deckObj')
            raw_decklist = player.get('decklist')

            if not deck_obj and not raw_decklist:
                continue

            decks.append({
                'player_name': player.get('name', 'Unknown'),
                'tournament_name': tournament_name,
                'standing': player.get('standing', 'Unknown'),
                'wins': player.get('wins', 0),
                'draws': player.get('draws', 0),
                'losses': player.get('losses', 0),
                'deck_obj': deck_obj,
                'decklist': raw_decklist,
            })

    return decks

def sanitize_filename(filename):
    """Remove invalid characters from filename"""
    return re.sub(r'[<>:"/\\|?*]', '', filename)

def parse_decklist(deck_info):
    """
    Convert TopDeck deckObj into mtgsim-compatible deck format.
    """

    deck_obj = deck_info.get("deck_obj")

    #
    # BEST PATH:
    # Use structured deckObj
    #

    if deck_obj:

        output = []

        commanders = deck_obj.get("Commanders", {})
        mainboard = deck_obj.get("Mainboard", {})

        if commanders:
            output.append("COMMANDER:")

            for card_name, card_data in commanders.items():
                count = card_data.get("count", 1)
                output.append(f"{count} {card_name}")

            output.append("")

        output.append("DECK:")

        for card_name, card_data in sorted(mainboard.items()):
            count = card_data.get("count", 1)
            output.append(f"{count} {card_name}")
        return "\n".join(output)

    #
    # FALLBACK:
    # Raw text decklist
    #

    raw = deck_info.get("decklist", "")

    if not raw:
        return ""

    return raw

def save_deck(deck_info, output_dir, dry_run=False, quiet=False):
    try:
        player_name = deck_info['player_name']
        tournament_name = deck_info['tournament_name']
        raw_decklist = deck_info['decklist']

        decklist = parse_decklist(deck_info)

        if not decklist.strip():
            print(f"  ✗ Empty deck after parsing for {player_name}")
            return False

        filename = (
            f"{sanitize_filename(tournament_name)}-"
            f"{sanitize_filename(player_name)}"
        )

        filename = filename[:120] + ".txt"

        filepath = output_dir / filename

        if filepath.exists():
            if not quiet:
                print(f"  ✓ Already exists: {filename}")
            return True

        metadata = f"""// PLAYER: {player_name}
// TOURNAMENT: {tournament_name}
// RECORD: {deck_info.get('wins', 0)}-{deck_info.get('losses', 0)}-{deck_info.get('draws', 0)}
// STANDING: {deck_info.get('standing', 'Unknown')}

"""

        final_output = metadata + decklist + "\n"

        if dry_run:
            print(f"  [DRY RUN] Would save: {filename}")
        else:
            filepath.write_text(final_output, encoding='utf-8')

            if not quiet:
                print(f"  ✓ Saved: {filename}")

        return True

    except Exception as e:
        print(
            f"  ✗ Error saving deck for "
            f"{deck_info.get('player_name', 'Unknown')}: {e}"
        )
        return False

def main():
    parser = argparse.ArgumentParser(
        description='Download EDH tournament decklists from TopDeck.gg',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
ENVIRONMENT VARIABLES:
  TOPDECK_API_KEY    Your TopDeck.gg API key (required)
                     Get one free at: https://topdeck.gg/account

EXAMPLES:
  # Default: last 30 days, 20+ participants
  %(prog)s

  # Last 60 days, 50+ participants
  %(prog)s --days 60 --min-participants 50

  # Custom output directory
  %(prog)s --output ~/edh-decks/

  # Preview without saving
  %(prog)s --dry-run

  # Quiet mode (no "already exists" messages)
  %(prog)s --quiet
        """
    )
    
    parser.add_argument(
        '--days',
        type=int,
        default=30,
        metavar='N',
        help='How many days back to search (default: 30)'
    )
    
    parser.add_argument(
        '--min-participants',
        type=int,
        default=20,
        metavar='N',
        help='Minimum tournament participant count (default: 20)'
    )
    
    parser.add_argument(
        '--output',
        type=Path,
        default=Path('decks/edh'),
        metavar='DIR',
        help='Output directory for deck files (default: decks/edh)'
    )
    
    parser.add_argument(
        '--dry-run',
        action='store_true',
        help='Preview what would be downloaded without saving'
    )
    
    parser.add_argument(
        '--quiet',
        action='store_true',
        help='Suppress "already exists" messages'
    )
    
    parser.add_argument(
        '--verbose',
        action='store_true',
        help='Show additional details during download'
    )
    
    args = parser.parse_args()
    
    # Create output directory
    if not args.dry_run:
        args.output.mkdir(parents=True, exist_ok=True)
    
    print(f"Output directory: {args.output.resolve()}\n")
    
    # Fetch tournaments
    tournaments = get_edh_tournaments(days_back=args.days, participant_min=args.min_participants)
    
    if not tournaments:
        print("No tournaments found")
        return
    
    # Extract decklists
    print(f"\nExtracting decklists from standings...")
    decks = extract_decks_from_tournaments(tournaments)
    print(f"Found {len(decks)} decklists\n")
    
    if args.verbose:
        print(f"Starting to process {len(decks)} decks...\n")
    
    # Save decks
    total = len(decks)
    success = 0
    failed = 0
    
    for i, deck in enumerate(decks, 1):
        if args.verbose or not args.quiet:
            print(f"[{i}/{total}] {deck['tournament_name']} - {deck['player_name']}")
        
        if save_deck(deck, args.output, dry_run=args.dry_run, quiet=args.quiet):
            success += 1
        else:
            failed += 1
    
    # Summary
    print(f"\n{'='*60}")
    if args.dry_run:
        print(f"DRY RUN - Would have:")
    else:
        print(f"Download complete!")
    print(f"  ✓ Successful: {success}")
    print(f"  ✗ Failed: {failed}")
    print(f"  Total: {total}")
    print(f"{'='*60}")

if __name__ == '__main__':
    main()
