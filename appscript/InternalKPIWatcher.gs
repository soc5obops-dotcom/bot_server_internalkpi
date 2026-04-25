const SHEET_NAME = 'Internal_kpi';
const WATCH_RANGE = 'S15:T39';
const HASH_PROPERTY = 'internal_kpi_watch_hash';
const SERVER_URL_PROPERTY = 'internal_kpi_server_url';
const WEBHOOK_SECRET_PROPERTY = 'internal_kpi_webhook_secret';

function installInternalKpiWatcher() {
  ScriptApp.newTrigger('pollInternalKpi')
    .timeBased()
    .everyMinutes(1)
    .create();
}

function configureInternalKpiWatcher(serverUrl, webhookSecret) {
  PropertiesService.getScriptProperties().setProperties({
    [SERVER_URL_PROPERTY]: serverUrl,
    [WEBHOOK_SECRET_PROPERTY]: webhookSecret,
  });
  initializeInternalKpiHash();
}

function initializeInternalKpiHash() {
  const values = SpreadsheetApp.getActive()
    .getSheetByName(SHEET_NAME)
    .getRange(WATCH_RANGE)
    .getDisplayValues();
  PropertiesService.getScriptProperties().setProperty(HASH_PROPERTY, hashValues_(values));
}

function pollInternalKpi() {
  const lock = LockService.getScriptLock();
  if (!lock.tryLock(5000)) {
    return;
  }

  try {
    const props = PropertiesService.getScriptProperties();
    const serverUrl = props.getProperty(SERVER_URL_PROPERTY);
    const webhookSecret = props.getProperty(WEBHOOK_SECRET_PROPERTY);
    if (!serverUrl || !webhookSecret) {
      throw new Error('Run configureInternalKpiWatcher(serverUrl, webhookSecret) first.');
    }

    const values = SpreadsheetApp.getActive()
      .getSheetByName(SHEET_NAME)
      .getRange(WATCH_RANGE)
      .getDisplayValues();
    const nextHash = hashValues_(values);
    const previousHash = props.getProperty(HASH_PROPERTY);

    if (!previousHash) {
      props.setProperty(HASH_PROPERTY, nextHash);
      return;
    }
    if (nextHash === previousHash) {
      return;
    }

    props.setProperty(HASH_PROPERTY, nextHash);
    UrlFetchApp.fetch(serverUrl, {
      method: 'post',
      contentType: 'application/json',
      headers: {
        'X-KPI-Webhook-Secret': webhookSecret,
      },
      payload: JSON.stringify({
        sheet: SHEET_NAME,
        watch_range: WATCH_RANGE,
        changed_at: new Date().toISOString(),
      }),
      muteHttpExceptions: true,
    });
  } finally {
    lock.releaseLock();
  }
}

function hashValues_(values) {
  const bytes = Utilities.computeDigest(
    Utilities.DigestAlgorithm.SHA_256,
    JSON.stringify(values),
    Utilities.Charset.UTF_8
  );
  return bytes.map(function(byte) {
    const normalized = byte < 0 ? byte + 256 : byte;
    return ('0' + normalized.toString(16)).slice(-2);
  }).join('');
}
