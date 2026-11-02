Feature: Reviewing how things look
  Screenshots recorded with each run show what changed in how the product looks. A person approves a change in
  the look like a change in behavior, and what they approved is what later changes are compared with.

  @proposed
  Scenario: A change shows which screenshots look different
    Given a project records a screenshot of each component or screen with its tests
    And a change alters how one of them looks
    When I open the change
    Then I see that one of them looks different
    And the screenshots that look the same are not in the way

  @proposed
  Scenario: Comparing before and after
    Given a change alters how a button looks
    When I open its screenshot
    Then I can compare the approved look and the new one, side by side or one over the other
    And the parts that changed are highlighted

  @proposed
  Scenario: The same thing in its variants
    Given a project records each screenshot in light and dark, and at phone and desktop widths
    When a change alters how a list row looks
    Then its variants are shown together, with the ones that changed marked

  @proposed
  Scenario: Approving a change in the look
    Given a change alters how a button looks
    When I approve the new look
    Then later changes are compared with the look I approved
    And the approval records who approved it

  @proposed
  Scenario: A look nobody approved holds a release
    Given a change alters how a button looks and nobody has approved it
    When I check whether the change can be released
    Then the check fails and names the screenshot that changed

  @proposed
  Scenario: A new screenshot needs a first approval
    Given a change adds a screenshot of a new component
    When I open the change
    Then I see it as new, with nothing to compare it with
    And it waits for my approval like a change
