# Arbatron

A pipeline for finding arbitrage opportunities on Polymarket.

It crawls events from the Polymarket API, filters them, embeds their titles,
and stores them in Postgres (pgvector). Similar events are then clustered
together and sent to Gemini, which looks for structural arbitrage between
markets (e.g. mutually exclusive outcomes).

## Setup

1. Start Postgres + Adminer:

   ```
   docker-compose up -d
   ```

2. Create a `.env` file with:

   ```
   GEMINI_KEY=your_key_here
   ```

3. Run:

   ```
   go run ./cmd
   ```

## Status

Only the first stage of the arbitrage discovery pipeline is wired up
(finding candidate clusters). The second stage, which evaluates real
market prices for ROI and risk, is written but not yet called from
`main.go`. See `TODO.txt` for what's next.
