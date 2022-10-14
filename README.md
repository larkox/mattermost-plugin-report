# Reporter

This plugin helps to report non permitted words in certain teams.

# Motivation

Sometimes, there is sensitive data that we don't want to share outside of a closed circle (e.g. an specific team).
This plugin can help tracking possible leaks by reporting them as soon as they happen.

# Usage

All the configuration is done in the system console.
You have to define:
- The terms that you don't want to be used in certain teams.
- The teams where you don't want those terms to be used.
- The users the report will go to.
- The channels the report will go to.

If no terms or no teams are defined, no reports will be generated. Also, if no users nor channels are defined, no reports will be generated.

Reports will not be done for messages in Direct Messages nor Group Messages.
