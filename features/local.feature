Feature: Using Shipwake without touching a repository
  A project can be followed from outside: its requirements, results and settings live in a
  workspace of my own, and the repository never learns Shipwake exists.

  @proposed
  Scenario: Setting up leaves the repository as it was
    Given a repository I don't want to change
    When I set it up in local mode
    Then its files, branches, tags and Git settings are as they were
    And its settings, requirements and results are in a workspace of its own on my machine

  @proposed
  Scenario: Approving commits to the workspace
    Given scenarios proposed for a project in local mode
    When I approve them in Shipwake
    Then the approval is a commit in the project's workspace that names me
    And the repository has no new commits

  @proposed
  Scenario: Runs sit with the repository's commits
    Given a project in local mode
    When I record a run of its tests
    Then the run is kept in the project's workspace
    And it is shown on the repository commit it tested

  @proposed
  Scenario: Opening every project at once
    Given two projects in local mode and one repository set up the usual way
    When I start Shipwake without naming a repository
    Then all three projects are shown
