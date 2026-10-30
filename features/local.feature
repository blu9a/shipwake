Feature: Using Shipwake without touching a repository
  Local mode: Shipwake reads a repository and writes nothing into it. The project's requirements,
  settings, results and recordings live in a workspace of its own on my machine, and deleting the
  workspace removes every trace. When I'm ready, the repository adopts them.

  Scenario: Opening a repository by its path
    Given a repository on my machine that Shipwake has never seen
    When I open it in Shipwake by its path
    Then its timeline is shown
    And its files, branches, tags and Git settings are as they were

  Scenario: Approving commits to the workspace
    Given scenarios proposed for a project in local mode
    When I approve them in Shipwake
    Then the approval is a commit in the project's workspace that names me
    And the repository has no new commits

  Scenario: Runs are kept in the workspace
    Given a project in local mode
    When I record a run of its tests, with a recording
    Then the run and its recording are kept in the project's workspace
    And the repository has no new branch
    And the run is shown on the repository commit it tested

  Scenario: Requirements follow the repository's branch
    Given a project in local mode with scenarios proposed for the branch feature/tip
    When the repository has feature/tip checked out
    Then the timeline of feature/tip shows the proposal, and it can be approved
    And the trunk's timeline shows nothing proposed

  Scenario: Opening every project at once
    Given two projects in local mode and one repository set up the usual way
    When I start Shipwake without naming a repository
    Then all three projects are shown

  Scenario: Giving an agent what it needs
    Given a project in local mode
    When I open the agent prompt
    Then it names the skill, the repository, its workspace and the checked-out branch
    And I can copy it with one click

  Scenario: Adopting the requirements into the repository
    Given a project in local mode with approved scenarios and recorded runs
    When I adopt it into the repository
    Then the repository has one new commit, made as me, with the requirements and settings
    And its message lists what was approved in local mode, and by whom
    And the workspace is kept as it was

  @proposed
  Scenario: An approval in local mode shows on the timeline
    Given scenarios proposed for a project in local mode
    When I approve them in Shipwake
    Then the timeline shows the approval, marked as kept in the workspace
    And it sits above the repository commit that was checked out when I approved
