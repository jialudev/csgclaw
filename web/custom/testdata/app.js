// This UI is mounted at the application root. Relative URLs preserve gateway prefixes.
// A hash router keeps this base stable; history routers must explicitly configure their base.
const appBase = new URL("./", window.location.href);
try {
  const version = await fetch(new URL("api/v1/version", appBase));
  if (!version.ok) throw new Error(`Backend version: ${version.status}`);
  document.querySelector("#result").textContent = JSON.stringify(await version.json(), null, 2);
} catch (error) {
  document.querySelector("#result").textContent = error.message;
}
