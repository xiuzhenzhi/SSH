import test from 'node:test';
import assert from 'node:assert/strict';
import { createResponseCache } from '../src/lib/responseCache.ts';
import { requestJSON, HTTPError } from '../src/lib/api.ts';

test('cached pages expire, stay bounded and cannot be restored by old sessions', () => {
  let clock=0;const cache=createResponseCache(2,()=>clock);
  cache.set('/servers?page=1',{items:['first']});
  assert.deepEqual(cache.get('/servers?page=1'),{items:['first']});
  assert.equal(cache.get('/servers?page=2'),undefined);
  clock=16000;assert.equal(cache.get('/servers?page=1'),undefined);
  assert.ok(cache.get('/servers?page=1',120000));
  cache.set('/events',{});cache.set('/alerts',{});
  assert.equal(cache.get('/servers?page=1',120000),undefined);
  const epoch=cache.epoch;cache.clear();cache.set('/servers',{},epoch);
  assert.equal(cache.get('/servers'),undefined);
});

test('deadline covers stalled response bodies and caller cancellation stays distinct', async t => {
  t.mock.method(globalThis,'fetch',async(_url,{signal})=>({ok:true,status:200,json:()=>new Promise((_,reject)=>{
    if(signal.aborted)reject(signal.reason);else signal.addEventListener('abort',()=>reject(signal.reason),{once:true});
  })}));
  await assert.rejects(requestJSON('/slow','GET',undefined,undefined,20),/连接超时/);
  const controller=new AbortController();
  const request=requestJSON('/cancel','GET',undefined,controller.signal,1000);controller.abort();
  await assert.rejects(request,{name:'AbortError'});
});

test('unauthorized responses retain status and malformed successes are rejected', async t => {
  const mock=t.mock.method(globalThis,'fetch',async()=>new Response(JSON.stringify({error:'请登录'}),{status:401}));
  await assert.rejects(requestJSON('/auth/me'),e=>e instanceof HTTPError&&e.status===401);
  mock.mock.mockImplementation(async()=>new Response('{}'));
  await assert.rejects(requestJSON('/servers'),/格式异常/);
});
