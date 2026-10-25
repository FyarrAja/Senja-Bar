const test = require('node:test');
const assert = require('node:assert/strict');
const create = require('../cmd/samar/ui/commands.js');
const initial = {style:1,rounded:true,startup:true};
function harness() {
  const sent=[], painted=[];
  return {sent,painted,q:create(initial,(state,error,pending)=>painted.push({state,error,pending}),c=>sent.push(c))};
}
test('rapid toggle reversal keeps the final intention through delayed acknowledgements',()=>{
  const {q,sent,painted}=harness();
  q.choose('rounded',false); q.choose('rounded',true);
  assert.deepEqual(sent,[105]);
  assert.equal(painted.at(-1).state.rounded,true);
  q.acknowledge({...initial,rounded:false});
  assert.deepEqual(sent,[105,105]);
  assert.equal(painted.at(-1).state.rounded,true);
  q.acknowledge(initial);
  assert.equal(painted.at(-1).pending,false);
});
test('mode clicks respond immediately and coalesce redundant engine work',()=>{
  const {q,sent,painted}=harness();
  q.choose('style',2); q.choose('style',3); q.choose('style',4);
  assert.deepEqual(sent,[101]);
  assert.equal(painted.at(-1).state.style,4);
  q.acknowledge({...initial,style:2});
  assert.deepEqual(sent,[101,103]);
  assert.equal(painted.at(-1).state.style,4);
  q.acknowledge({...initial,style:4});
  assert.equal(painted.at(-1).pending,false);
});
test('engine failure rolls optimistic state back instead of retrying forever',()=>{
  const {q,sent,painted}=harness();
  q.choose('style',2); q.choose('startup',false);
  q.acknowledge(initial,'Not applied');
  assert.deepEqual(sent,[101]);
  assert.deepEqual(painted.at(-1).state,initial);
  assert.equal(painted.at(-1).error,'Not applied');
});
test('exit remains available while Windows is busy',()=>{
  const {q,sent}=harness();
  q.choose('style',2); q.action(111); q.action(111); q.action(140);
  assert.deepEqual(sent,[101,140]);
  q.acknowledge({...initial,style:2});
  assert.deepEqual(sent,[101,140,111]);
});
test('an unacknowledged state change cannot create a retry loop',()=>{
  const {q,sent,painted}=harness();
  q.choose('startup',false);
  q.acknowledge(initial);
  assert.deepEqual(sent,[110]);
  assert.deepEqual(painted.at(-1).state,initial);
  assert.ok(painted.at(-1).error);
});
