const GITHUB_API_TOKEN = getProperty('GITHUB_API_TOKEN');

function doPost(e) {
  if (!GITHUB_API_TOKEN) {
    throw 'GITHUB_API_TOKEN is empty'
  }
  const req = JSON.parse(e.postData.getDataAsString());
  if (req.action != 'closed' || !req.pull_request) {
    throw 'Not pull_request closed event. ' + JSON.stringify(req);
    Logger.log('Not pull_request closed event');
    return;
  }
  const data = {'event_type':'invite'};
  const options = {
    'method' : 'post',
    'contentType': 'application/json',
    'payload' : JSON.stringify(data),
    'headers': {
      'Authorization': 'token ' + GITHUB_API_TOKEN
    }
  };
  UrlFetchApp.fetch('https://api.github.com/repos/reviewdog/inviter/dispatches', options);
}

function getProperty(key) {
  return PropertiesService.getScriptProperties().getProperty(key);
}
