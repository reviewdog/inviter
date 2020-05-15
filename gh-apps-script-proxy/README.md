## GitHub Webhook Apps Script Proxy
This Apps script receives GitHub Webhook (especially for Organization webhook)
and call `repository_dispatch` API to kick the invite workflow in this repo.

### Set up
1. Deploy this Apps Script as webapp and get web app URL.
2. Set up GitHub token as a script property.
3. Create an organization webhook with Pull Request event and set the above URL.

### Maintainer
[@haya14busa](https://github.com/haya14busa)
