const fs = require('fs');
const path = require('path');
const assert = require('assert');

const appPath = path.join(__dirname, 'app.js');
const source = fs.readFileSync(appPath, 'utf8');

assert(
  source.includes('agentPublish: "Publish"') && source.includes('agentPublish: "发布"'),
  'agent detail page must expose a localized publish action',
);
assert(
  source.includes('async function publishAgentPage()'),
  'frontend must define a publish handler for the agent detail page',
);
assert(
  source.includes('fetch("/api/v1/hub/templates", {'),
  'publish handler must call the hub template publish API',
);
assert(
  source.includes('agent_id: selectedAgentForPage.id'),
  'publish handler must publish the currently selected agent',
);
assert(
  source.includes('setSelectedHubTemplateId(published.id);'),
  'publish handler must focus the newly published template in Hub',
);
assert(
  source.includes('preview-action-button-primary entity-toolbar-publish'),
  'publish button must use the primary blue styling and toolbar right alignment class',
);
assert(
  source.includes('const canPublish = runtimeKind === "picoclaw_sandbox" || runtimeKind === "openclaw_sandbox";'),
  'publish button must only be available for picoclaw_sandbox or openclaw_sandbox agents',
);
