Feature: Getting started
  One command sets a repository up, and an agent can run it: in local mode, nothing is written into the
  repository.

  Scenario: Setting up in local mode from the terminal
    Given a repository on my machine that Shipwake has never seen
    When I run "shipwake init --local --write" in it
    Then it is in local mode, its settings written and committed in its workspace
    And the repository is as it was

  Scenario: Setting up a project already in local mode
    Given a project in local mode
    When I run "shipwake init --write" in its repository
    Then its settings are written in its workspace, not in the repository
