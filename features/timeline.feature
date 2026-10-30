Feature: Seeing what every change did
  Someone responsible for a product sees, for each change, what was tested and how it went,
  without reading its code.

  @proposed
  Scenario: A run sits with the commit it tested
    Given a project whose newest commit came after an older one
    And a test run on the older commit, recorded after the newest commit
    When I open the project's timeline
    Then the run appears directly above the older commit
    And it names the commit it tested

  @proposed
  Scenario: A failing test is visible on its commit
    Given a test run with one failing test on a commit
    When I open the project's timeline
    Then that commit's run shows one failure
    And opening the run shows the failing test with its error

  @proposed
  Scenario: A release shows what it records
    Given a release whose changelog lists a new feature and a fix
    When I open the project's timeline
    Then the release shows the feature and the fix as its changelog says
