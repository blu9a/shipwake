Feature: Approving requirements
  Whoever is responsible decides what gets built: proposed scenarios and metrics wait for them,
  and approving them is their own commit.

  Scenario: Proposals wait at the top of the timeline
    Given scenarios proposed for a feature
    When I open the project's timeline
    Then the proposal is shown above everything else
    And it can be opened to read its scenarios

  Scenario: Approving records who approved
    Given scenarios proposed for a feature
    When I approve them in Shipwake
    Then they are no longer proposed
    And the commit that approved them names me as the approver

  Scenario: A metric Shipwake can't read can't be approved
    Given a proposed metric with a line Shipwake doesn't understand
    When I open the proposal
    Then that line is marked with the reason
    And approving it is not possible

  @proposed
  Scenario: Each branch shows its own requirements
    Given scenarios proposed on a branch that is not merged
    When I open the timeline of the trunk
    Then nothing is shown as proposed
    And the branch's timeline shows the proposal
    And it can be approved only while that branch is checked out
