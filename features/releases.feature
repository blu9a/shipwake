Feature: Releasing
  A release says what changed for the product, is checked before it is approved,
  and is recorded in the repository.

  Scenario: The changelog is drafted from the changes
    Given commits since the last release: a feature, a fix and a test
    When I prepare a new release and assemble the draft
    Then the notes list the feature and the fix
    And they don't list the test

  Scenario: Approving a release records it
    Given checks that pass on the newest commit
    When I approve a release with its notes
    Then the changelog has a section for the new version
    And the version is tagged with me as the approver

  Scenario: Nothing to release
    Given every change is already released
    When I open the new release page
    Then it says there is nothing new since the last release
