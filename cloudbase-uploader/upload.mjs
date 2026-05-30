import fs from 'node:fs'
import cloudbase from '@cloudbase/node-sdk'

const [, , localFilePath, cloudPath, envId] = process.argv
const secretId = process.env.COS_SECRET_ID || process.env.TENCENTCLOUD_SECRETID || ''
const secretKey = process.env.COS_SECRET_KEY || process.env.TENCENTCLOUD_SECRETKEY || ''
const sessionToken = process.env.COS_SESSION_TOKEN || process.env.TENCENTCLOUD_SESSIONTOKEN || ''

if (!localFilePath || !cloudPath || !envId) {
  console.error(JSON.stringify({ error: 'missing arguments', localFilePath, cloudPath, envId }))
  process.exit(1)
}

if (!secretId || !secretKey) {
  console.error(JSON.stringify({
    error: 'missing credentials',
    hasSecretId: Boolean(secretId),
    hasSecretKey: Boolean(secretKey),
    hasSessionToken: Boolean(sessionToken)
  }))
  process.exit(1)
}

const app = cloudbase.init({
  env: envId,
  secretId,
  secretKey,
  sessionToken: sessionToken || undefined
})

try {
  const uploadResult = await app.uploadFile({
    cloudPath,
    fileContent: fs.createReadStream(localFilePath)
  })

  const fileID = uploadResult?.fileID || uploadResult?.fileId || uploadResult?.fildID || uploadResult?.fildId
  if (!fileID) {
    throw new Error(`upload succeeded but fileID missing: ${JSON.stringify(uploadResult)}`)
  }

  const tempResult = await app.getTempFileURL({
    fileList: [{ fileID, maxAge: 86400 }]
  })

  const fileItem = Array.isArray(tempResult?.fileList) ? tempResult.fileList[0] : null
  const tempFileURL = fileItem?.tempFileURL || ''

  process.stdout.write(JSON.stringify({
    fileID,
    tempFileURL,
    uploadResult,
    tempResult
  }))
} catch (error) {
  console.error(JSON.stringify({
    error: error instanceof Error ? error.message : String(error),
    debug: {
      argvEnvId: envId,
      cloudEnvId: process.env.CLOUD_ENV_ID || '',
      tcbEnv: process.env.TCB_ENV || '',
      tcbEnvId: process.env.TCB_ENVID || '',
      wxCloudEnv: process.env.WX_CLOUD_ENV || '',
      envId: process.env.ENV_ID || '',
      hasSecretId: Boolean(secretId),
      hasSecretKey: Boolean(secretKey),
      hasSessionToken: Boolean(sessionToken)
    }
  }))
  process.exit(1)
}
