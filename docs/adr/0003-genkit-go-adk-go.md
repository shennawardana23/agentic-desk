# ADR-0003: Genkit Go 1.0 GA + ADK Go 2.0 GA

Status: Accepted — 2026-07-06

## Context

The platform needs an LLM orchestration framework and a multi-agent framework, in Go. Genkit Go reached its 1.0 GA (stable, semver-locked) release; ADK Go reached its 2.0 GA release (June 30 2026) with a graph-based workflow engine and built-in human-in-the-loop and durable pause/resume support. Both were explicitly required by the user, and both were verified as GA/stable at design time rather than assumed.

## Decision

Use Genkit Go 1.0 as the LLM/flow orchestration framework (flows, Dotprompt, middleware, MCP plugin). Use ADK Go 2.0 as the multi-agent framework starting with sub-project 3, with its graph/node-edge model and durable pause/resume adopted now as the substrate for the Agent Loop primitive's HITL escalation.

## Consequences

- Both frameworks are stable/GA, not preview APIs — reduces risk of building against a moving target.
- ADK Go 2.0's durable pause/resume is a load-bearing dependency for HITL escalation; its exact API shape is an open verification item (see design doc Section 10) that must be confirmed against live docs before that code is written, not assumed from the release announcement.
