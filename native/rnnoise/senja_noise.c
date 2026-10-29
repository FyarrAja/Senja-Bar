#include <stdint.h>
#include <stdlib.h>
#include <math.h>
#include "rnnoise.h"

typedef struct {
  DenoiseState *model;
  float input[480], output[480];
  float probability;
  unsigned position;
} SenjaNoise;

__declspec(dllexport) unsigned senja_noise_abi(void) { return 1; }
__declspec(dllexport) void *senja_noise_create(void) {
  if (rnnoise_get_frame_size()!=480) return NULL;
  SenjaNoise *state=(SenjaNoise*)calloc(1,sizeof(SenjaNoise));
  if (!state) return NULL;
  state->model=rnnoise_create(NULL);
  if (!state->model) {free(state);return NULL;}
  return state;
}
__declspec(dllexport) void senja_noise_destroy(void *opaque) {
  SenjaNoise *state=(SenjaNoise*)opaque;
  if (!state) return;
  rnnoise_destroy(state->model);
  free(state);
}
// Synchronous PCM interface: no Go pointer is retained. RNNoise expects the
// 16-bit PCM amplitude range as float, not samples normalised to [-1,1].
__declspec(dllexport) int senja_noise_process(void *opaque,int16_t *pcm,unsigned length) {
  SenjaNoise *state=(SenjaNoise*)opaque;
  if (!state || !pcm || !length) return 0;
  double speech=0;
  for (unsigned i=0;i<length;i++) {
    unsigned p=state->position;
    float emitted=state->output[p];
    speech+=state->probability;
    state->input[p]=(float)pcm[i];
    emitted=fminf(32767.f,fmaxf(-32768.f,emitted));
    pcm[i]=(int16_t)lrintf(emitted);
    if (++state->position==480) {
      state->probability=rnnoise_process_frame(state->model,state->output,state->input);
      state->position=0;
    }
  }
  return (int)lrint(1000.*speech/length);
}
