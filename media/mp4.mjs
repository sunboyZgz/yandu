import { createReadStream } from 'node:fs';
import { stat, rename, rm } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import path from 'node:path';

export function mediaError(code, message, details) {
  return Object.assign(new Error(message), { code, details });
}

export async function processMP4({ root, input, common, encode, run, probe, constraint = {} }) {
  const video = probe.streams.find(v => v.codec_type === 'video');
  const sourceAudio=probe.streams.find(v=>v.codec_type==='audio');
  const audioStart=Math.max(0,Number(sourceAudio?.start_time||0)-Number(probe.format?.start_time||0));
  const videoStart=Math.max(0,Number(video?.start_time||0)-Number(probe.format?.start_time||0));
  const originalDuration = Number(probe.format?.duration || video?.duration);
  if (!Number.isFinite(originalDuration) || originalDuration <= 0) throw mediaError('UNSUPPORTED_MEDIA', 'Source has no valid finite duration');
  const limit = constraint.maxFileBytes || 0;
  if (limit && (!Number.isSafeInteger(limit) || limit <= 0 || !['reject', 'keep-start'].includes(constraint.onOverflow))) throw mediaError('INVALID_MEDIA_RESULT', 'Invalid server output constraint');
  const output = path.join(root, 'video.mp4');
  const encodePrefix = async (target, duration) => {
    await run('ffmpeg', [...common, ...encode, ...(duration === undefined ? [] : ['-t', duration.toFixed(9)]), '-movflags', '+faststart', target]);
  };
  const inspect = async (file, decode = false, prefix = false) => {
    const data = JSON.parse(await run('ffprobe', ['-protocol_whitelist', 'file,pipe', '-f', 'mov', '-v', 'error', '-count_frames', '-show_streams', '-show_format', '-show_entries', 'format=duration,start_time:stream=codec_type,codec_name,nb_read_frames,duration,start_time', '-of', 'json', file], true));
    const v = data.streams.find(s => s.codec_type === 'video');
    const a = data.streams.find(s => s.codec_type === 'audio');
    const duration = Number(data.format?.duration);
    const frames=Number(v?.nb_read_frames);
    if (!v || !Number.isSafeInteger(frames) || frames<1) throw mediaError(prefix?'PREFIX_TOO_SHORT':'INVALID_MEDIA_RESULT','MP4 has no decoded video frames');
    const audioRequired=Boolean(sourceAudio)&&duration>audioStart+0.1;
    if (v.codec_name !== 'h264' || !Number.isFinite(duration) || duration <= 0 || duration > originalDuration + 0.25 || Math.abs(Number(data.format?.start_time || 0)) > 0.1 || (audioRequired && (!a || a.codec_name !== 'aac' || !Number.isSafeInteger(Number(a.nb_read_frames)) || Number(a.nb_read_frames)<1))) {
      throw mediaError('INVALID_MEDIA_RESULT', 'MP4 has invalid streams, start time, frames or duration');
    }
    if (decode) await run('ffmpeg', ['-nostdin', '-hide_banner', '-loglevel', 'error', '-xerror', '-err_detect', 'explode', '-threads', '2', '-protocol_whitelist', 'file,pipe', '-f', 'mov', '-i', file, '-map', '0:v:0', '-map', '0:a:0?', '-f', 'null', '-']);
    return { duration, bytes: (await stat(file)).size };
  };
  await encodePrefix(output);
  const fullBytes = (await stat(output)).size;
  let trimmed = false;
  if (limit && fullBytes > limit) {
    if (constraint.onOverflow !== 'keep-start') throw mediaError('OUTPUT_LIMIT_EXCEEDED', 'Full MP4 exceeds project file limit', { maxFileBytes: limit, fullCompressedBytes: fullBytes });
    const full = await inspect(output, true);
    await rm(output);
    const candidate = path.join(root, 'candidate.mp4');
    const best = path.join(root, 'best.mp4');
    let low = 0, high = Math.min(full.duration, originalDuration), found = false;
    // Byte size is measured for every candidate. The duration search is only a
    // heuristic; it never proves that an unmeasured candidate fits the budget.
    for (let attempt = 0; attempt < 12; attempt++) {
      const duration = (low + high) / 2;
      await encodePrefix(candidate, duration);
      const bytes = (await stat(candidate)).size;
      if (bytes > 0 && bytes <= limit) {
        try {
          const measured = await inspect(candidate, true, true);
          if (measured.duration >= originalDuration) throw mediaError('INVALID_MEDIA_RESULT', 'Trim did not shorten the source');
          await rm(best, { force: true }); await rename(candidate, best);
          found = true; low = duration;
        } catch (error) {
          if (error.code === 'PREFIX_TOO_SHORT') low=duration;
          else if(error.code==='INVALID_MEDIA_RESULT')high=duration;
          else throw error;
        }
      } else high = duration;
      await rm(candidate, { force: true });
    }
    if (!found) {
      const fraction = String(video.r_frame_rate || '30/1').split('/').map(Number);
      const fps = fraction[1] > 0 ? fraction[0] / fraction[1] : 30;
      const shortest = Math.min(originalDuration, videoStart + (Number.isFinite(fps) && fps > 0 ? 1 / fps : 1 / 30));
      await encodePrefix(candidate, shortest);
      if ((await stat(candidate)).size <= limit) {
        try { const measured=await inspect(candidate,true,true);if(measured.duration>=originalDuration)throw mediaError('INVALID_MEDIA_RESULT','Trim did not shorten the source');await rename(candidate, best); found = true; }
        catch (error) { if (!['INVALID_MEDIA_RESULT','PREFIX_TOO_SHORT'].includes(error.code)) throw error; }
      }
    }
    if (!found) throw mediaError('NO_VALID_TRIM', 'No valid playable prefix fits the project limit', { maxFileBytes: limit, fullCompressedBytes: fullBytes });
    await rename(best, output); await rm(candidate, { force: true }); trimmed = true;
  }
  const final = await inspect(output, true);
  if (limit && final.bytes > limit) throw mediaError('OUTPUT_LIMIT_EXCEEDED', 'Final MP4 exceeds project file limit', { maxFileBytes: limit, fullCompressedBytes: fullBytes, outputBytes: final.bytes });
  const hash = createHash('sha256'); for await (const chunk of createReadStream(output)) hash.update(chunk);
  return {
    originalDurationSeconds: originalDuration,
    fullCompressedBytes: fullBytes,
    outputDurationSeconds: final.duration,
    outputBytes: final.bytes,
    outputSha256: hash.digest('hex'),
    mime: 'video/mp4', trimmed,
    retainedRange: { startSeconds: 0, endSeconds: final.duration },
  };
}
