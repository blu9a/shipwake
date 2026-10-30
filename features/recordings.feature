Feature: Watching what a scenario did
  A scenario's recording shows the product, never an empty screen, so nobody watching thinks it froze.

  @proposed
  Scenario: A recording starts where the page shows something
    Given a scenario recorded while its page was still blank for the first second
    When I open the scenario's recording
    Then it shows the page from the start, not a blank screen

  @proposed
  Scenario: A blank moment holds the last picture
    Given a scenario recorded while its page went blank for a second in the middle
    When I play the recording through that second
    Then the page's last picture stays on screen until the page is back

  @proposed
  Scenario: A feature's video ends each scenario on its result
    Given a feature with two recorded scenarios
    When I watch the feature
    Then the end of the first scenario stays on screen a moment before the second begins
