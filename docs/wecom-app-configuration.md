# WeCom App Configuration Guide

This document describes how to configure the WeCom App (WeChat Work custom application) channel in MilliBee.

## Features

| Feature | Status |
|---------|--------|
| Receive messages (passive) | ✅ |
| Send messages (proactive) | ✅ |
| Direct messages | ✅ |
| Group chat | ❌ |

## Setup

### 1. WeCom Admin Console

1. Log in to the [WeCom Admin Console](https://work.weixin.qq.com/wework_admin)
2. Go to "App Management" and select your custom app
3. Note the following:
   - **AgentId**: shown on the app details page
   - **Secret**: click "View" to retrieve
4. Go to "My Enterprise" page and note the **CorpID**

### 2. Message Receiving Configuration

1. On the app details page, click "Set API Receiving" under "Receive Messages"
2. Fill in:
   - **URL**: `http://your-server:18792/webhook/wecom-app`
   - **Token**: randomly generated or custom (used for signature verification)
   - **EncodingAESKey**: click "Random Generate" to create a 43-character key
3. When you click "Save", WeCom will send a verification request

### 3. MilliBee Configuration

Add the following to `config.json`:

```json
{
  "channels": {
    "wecom_app": {
      "enabled": true,
      "corp_id": "wwxxxxxxxxxxxxxxxx",
      "corp_secret": "xxxxxxxxxxxxxxxxxxxxxxxx",
      "agent_id": 1000002,
      "token": "your_token",
      "encoding_aes_key": "your_encoding_aes_key",
      "webhook_host": "0.0.0.0",
      "webhook_port": 18792,
      "webhook_path": "/webhook/wecom-app",
      "allow_from": [],
      "reply_timeout": 5
    }
  }
}
```

## Troubleshooting

### 1. Callback URL Verification Failed

**Symptom**: WeCom shows verification failure when saving the API message receiver

**Checklist**:
- Confirm your server firewall allows port 18792
- Confirm `corp_id`, `token`, and `encoding_aes_key` are correct
- Check MilliBee logs for incoming requests

### 2. Message Decryption Failed

**Symptom**: `invalid padding size` error when receiving messages

**Cause**: WeCom uses non-standard PKCS7 padding (32-byte block size)

**Fix**: Ensure you are using the latest version of MilliBee, which handles this correctly.

### 3. Port Conflict

**Symptom**: Port already in use error on startup

**Fix**: Change `webhook_port` to another port, e.g. 18794

## Technical Details

### Encryption

- **Algorithm**: AES-256-CBC
- **Key**: EncodingAESKey Base64-decoded to 32 bytes
- **IV**: First 16 bytes of AESKey
- **Padding**: PKCS7 (32-byte block size, non-standard)
- **Message format**: XML

### Message Structure

Decrypted message format:
```
random(16B) + msg_len(4B) + msg + receiveid
```

For custom apps, `receiveid` is the `corp_id`.

## Debugging

Enable debug mode for detailed logs:

```bash
millibee gateway --debug
```

Key log tags:
- `wecom_app`: WeCom App channel logs
- `wecom_common`: encryption/decryption logs

## References

- [WeCom Official Docs - Receiving Messages](https://developer.work.weixin.qq.com/document/path/96211)
- [WeCom Official Crypto Library](https://github.com/sbzhu/weworkapi_golang)
