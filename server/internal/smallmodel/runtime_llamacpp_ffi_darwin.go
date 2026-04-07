package smallmodel

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

type llamaPos int32
type llamaToken int32
type llamaSeqID int32

type llamaModel struct{}
type llamaContext struct{}
type llamaVocab struct{}
type llamaSampler struct{}

type llamaBatch struct {
	NTokens int32
	Token   *llamaToken
	Embd    *float32
	Pos     *llamaPos
	NSeqID  *int32
	SeqID   **llamaSeqID
	Logits  *int8
}

type llamaModelTensorBuftOverride struct {
	Pattern string
	Buft    unsafe.Pointer
}

type llamaModelKVOverride struct {
	Tag int32
	Key [128]byte
	Val [128]byte
}

type llamaModelParams struct {
	Devices                  *unsafe.Pointer
	TensorBuftOverrides      *llamaModelTensorBuftOverride
	NGPULayers               int32
	SplitMode                int32
	MainGPU                  int32
	TensorSplit              *float32
	ProgressCallback         uintptr
	ProgressCallbackUserData unsafe.Pointer
	KVOverrides              *llamaModelKVOverride
	VocabOnly                bool
	UseMmap                  bool
	UseDirectIO              bool
	UseMlock                 bool
	CheckTensors             bool
	UseExtraBufts            bool
	NoHost                   bool
	NoAlloc                  bool
}

type llamaSamplerSeqConfig struct {
	SeqID   llamaSeqID
	Sampler *llamaSampler
}

type llamaContextParams struct {
	NCtx              uint32
	NBatch            uint32
	NUBatch           uint32
	NSeqMax           uint32
	NThreads          int32
	NThreadsBatch     int32
	RopeScalingType   int32
	PoolingType       int32
	AttentionType     int32
	FlashAttnType     int32
	RopeFreqBase      float32
	RopeFreqScale     float32
	YarnExtFactor     float32
	YarnAttnFactor    float32
	YarnBetaFast      float32
	YarnBetaSlow      float32
	YarnOrigCtx       uint32
	DefragThold       float32
	CBEval            unsafe.Pointer
	CBEvalUserData    unsafe.Pointer
	TypeK             int32
	TypeV             int32
	AbortCallback     unsafe.Pointer
	AbortCallbackData unsafe.Pointer
	Embeddings        bool
	OffloadKQV        bool
	NoPerf            bool
	OpOffload         bool
	SWAFull           bool
	KVUnified         bool
	Samplers          *llamaSamplerSeqConfig
	NSamplers         uintptr
}

type llamaSamplerChainParams struct {
	NoPerf bool
}

type llamaCppFFIBackend struct {
	once sync.Once
	err  error

	handle  uintptr
	libPath string

	modelMu   sync.Mutex
	modelPath string
	model     *llamaModel
	vocab     *llamaVocab

	backendInit               func()
	modelDefaultParams        func() llamaModelParams
	contextDefaultParams      func() llamaContextParams
	samplerChainDefaultParams func() llamaSamplerChainParams
	modelLoadFromFile         func(string, llamaModelParams) *llamaModel
	modelFree                 func(*llamaModel)
	initFromModel             func(*llamaModel, llamaContextParams) *llamaContext
	contextFree               func(*llamaContext)
	modelGetVocab             func(*llamaModel) *llamaVocab
	nCtx                      func(*llamaContext) uint32
	setNThreads               func(*llamaContext, int32, int32)
	batchInit                 func(int32, int32, int32) llamaBatch
	batchFree                 func(llamaBatch)
	decode                    func(*llamaContext, llamaBatch) int32
	samplerChainInit          func(llamaSamplerChainParams) *llamaSampler
	samplerChainAdd           func(*llamaSampler, *llamaSampler)
	samplerInitGreedy         func() *llamaSampler
	samplerInitTopK           func(int32) *llamaSampler
	samplerInitTopP           func(float32, uintptr) *llamaSampler
	samplerInitTemp           func(float32) *llamaSampler
	samplerInitDist           func(uint32) *llamaSampler
	samplerFree               func(*llamaSampler)
	samplerSample             func(*llamaSampler, *llamaContext, int32) llamaToken
	vocabEOS                  func(*llamaVocab) llamaToken
	vocabIsEOG                func(*llamaVocab, llamaToken) bool
	tokenize                  func(*llamaVocab, string, int32, *llamaToken, int32, bool, bool) int32
	tokenToPiece              func(*llamaVocab, llamaToken, *byte, int32, int32, bool) int32
}

func newLlamaCppFFIBackend() *llamaCppFFIBackend {
	return &llamaCppFFIBackend{}
}

func (b *llamaCppFFIBackend) EnsureLoaded() error {
	if b == nil {
		return fmt.Errorf("llama ffi backend is nil")
	}
	b.once.Do(func() {
		b.err = b.load()
	})
	return b.err
}

func (b *llamaCppFFIBackend) load() error {
	_, libFile, err := resolveLlamaCppSharedLibFile("llama")
	if err != nil {
		return err
	}
	handle, err := purego.Dlopen(libFile, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("load llama shared library %q: %w", libFile, err)
	}
	b.handle = handle
	b.libPath = libFile

	for _, binding := range []struct {
		name     string
		required bool
		target   any
	}{
		{name: "llama_backend_init", required: true, target: &b.backendInit},
		{name: "llama_model_default_params", required: true, target: &b.modelDefaultParams},
		{name: "llama_context_default_params", required: true, target: &b.contextDefaultParams},
		{name: "llama_sampler_chain_default_params", required: true, target: &b.samplerChainDefaultParams},
		{name: "llama_model_load_from_file", required: false, target: &b.modelLoadFromFile},
		{name: "llama_model_free", required: true, target: &b.modelFree},
		{name: "llama_init_from_model", required: true, target: &b.initFromModel},
		{name: "llama_free", required: true, target: &b.contextFree},
		{name: "llama_model_get_vocab", required: true, target: &b.modelGetVocab},
		{name: "llama_n_ctx", required: true, target: &b.nCtx},
		{name: "llama_set_n_threads", required: true, target: &b.setNThreads},
		{name: "llama_batch_init", required: true, target: &b.batchInit},
		{name: "llama_batch_free", required: true, target: &b.batchFree},
		{name: "llama_decode", required: true, target: &b.decode},
		{name: "llama_sampler_chain_init", required: true, target: &b.samplerChainInit},
		{name: "llama_sampler_chain_add", required: true, target: &b.samplerChainAdd},
		{name: "llama_sampler_init_greedy", required: true, target: &b.samplerInitGreedy},
		{name: "llama_sampler_init_top_k", required: true, target: &b.samplerInitTopK},
		{name: "llama_sampler_init_top_p", required: true, target: &b.samplerInitTopP},
		{name: "llama_sampler_init_temp", required: true, target: &b.samplerInitTemp},
		{name: "llama_sampler_init_dist", required: true, target: &b.samplerInitDist},
		{name: "llama_sampler_free", required: true, target: &b.samplerFree},
		{name: "llama_sampler_sample", required: true, target: &b.samplerSample},
		{name: "llama_vocab_eos", required: true, target: &b.vocabEOS},
		{name: "llama_vocab_is_eog", required: false, target: &b.vocabIsEOG},
		{name: "llama_tokenize", required: true, target: &b.tokenize},
		{name: "llama_token_to_piece", required: true, target: &b.tokenToPiece},
	} {
		if err := registerLlamaPuregoFunc(handle, binding.name, binding.target); err != nil {
			if !binding.required {
				continue
			}
			if binding.name == "llama_model_load_from_file" {
				if legacyErr := registerLlamaPuregoFunc(handle, "llama_load_model_from_file", binding.target); legacyErr == nil {
					continue
				}
			}
			return err
		}
	}

	b.backendInit()
	return nil
}

func (b *llamaCppFFIBackend) Generate(ctx context.Context, req llamaCppDirectGenerateRequest) (string, error) {
	if err := b.EnsureLoaded(); err != nil {
		return "", err
	}
	if len(req.Images) > 0 {
		return "", errLlamaCppDirectUnsupported
	}
	model, vocab, err := b.ensureModel(req.ModelPath)
	if err != nil {
		return "", err
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return "", fmt.Errorf("empty prompt")
	}
	promptTokens, err := b.tokenizePrompt(vocab, prompt)
	if err != nil {
		return "", err
	}
	if len(promptTokens) == 0 {
		return "", fmt.Errorf("prompt tokenization returned no tokens")
	}

	cparams := b.contextDefaultParams()
	batchSize := len(promptTokens)
	if batchSize < 1 {
		batchSize = 1
	}
	cparams.NBatch = uint32(batchSize)
	cparams.NUBatch = uint32(batchSize)
	cparams.NSeqMax = 1
	threads := llamaDirectDefaultThreads()
	cparams.NThreads = int32(threads)
	cparams.NThreadsBatch = int32(threads)

	llamaCtx := b.initFromModel(model, cparams)
	if llamaCtx == nil {
		return "", fmt.Errorf("llama_init_from_model returned nil")
	}
	defer b.contextFree(llamaCtx)
	b.setNThreads(llamaCtx, int32(threads), int32(threads))

	nCtx := int(b.nCtx(llamaCtx))
	if nCtx > 0 {
		remaining := nCtx - len(promptTokens)
		if remaining <= 0 {
			return "", fmt.Errorf("prompt exceeds llama context size (%d tokens >= %d)", len(promptTokens), nCtx)
		}
		if req.MaxTokens > remaining {
			req.MaxTokens = remaining
		}
	}
	if req.MaxTokens <= 0 {
		return "", nil
	}

	promptBatch := b.batchInit(int32(len(promptTokens)), 0, 1)
	defer b.batchFree(promptBatch)
	if err := fillLlamaBatch(&promptBatch, promptTokens, 0); err != nil {
		return "", err
	}
	if code := b.decode(llamaCtx, promptBatch); code != 0 {
		return "", fmt.Errorf("llama_decode(prompt) failed: %d", code)
	}

	sampler, err := b.newSampler(req.Temperature)
	if err != nil {
		return "", err
	}
	defer b.samplerFree(sampler)

	tokenBatch := b.batchInit(1, 0, 1)
	defer b.batchFree(tokenBatch)

	var out strings.Builder
	nPast := len(promptTokens)
	eosToken := b.vocabEOS(vocab)

	for i := 0; i < req.MaxTokens; i++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		token := b.samplerSample(sampler, llamaCtx, -1)
		if token == eosToken || (b.vocabIsEOG != nil && b.vocabIsEOG(vocab, token)) {
			break
		}
		piece, err := b.tokenPiece(vocab, token)
		if err != nil {
			return "", err
		}
		out.WriteString(piece)
		if err := fillLlamaBatch(&tokenBatch, []llamaToken{token}, nPast); err != nil {
			return "", err
		}
		if code := b.decode(llamaCtx, tokenBatch); code != 0 {
			return "", fmt.Errorf("llama_decode(token) failed: %d", code)
		}
		nPast++
	}

	return strings.TrimSpace(out.String()), nil
}

func (b *llamaCppFFIBackend) ensureModel(modelPath string) (*llamaModel, *llamaVocab, error) {
	cleanPath := strings.TrimSpace(modelPath)
	if cleanPath == "" {
		return nil, nil, fmt.Errorf("empty llama model path")
	}
	b.modelMu.Lock()
	defer b.modelMu.Unlock()

	if b.model != nil && b.modelPath == cleanPath && b.vocab != nil {
		return b.model, b.vocab, nil
	}
	if b.model != nil {
		b.modelFree(b.model)
		b.model = nil
		b.vocab = nil
		b.modelPath = ""
	}

	params := b.modelDefaultParams()
	model := b.modelLoadFromFile(cleanPath, params)
	if model == nil {
		return nil, nil, fmt.Errorf("llama_model_load_from_file(%q) returned nil", cleanPath)
	}
	vocab := b.modelGetVocab(model)
	if vocab == nil {
		b.modelFree(model)
		return nil, nil, fmt.Errorf("llama_model_get_vocab(%q) returned nil", cleanPath)
	}
	b.model = model
	b.vocab = vocab
	b.modelPath = cleanPath
	return b.model, b.vocab, nil
}

func (b *llamaCppFFIBackend) tokenizePrompt(vocab *llamaVocab, prompt string) ([]llamaToken, error) {
	size := len(prompt) + 8
	if size < 32 {
		size = 32
	}
	buf := make([]llamaToken, size)
	n := b.tokenize(vocab, prompt, int32(len(prompt)), &buf[0], int32(len(buf)), true, false)
	if n < 0 {
		need := int(-n)
		if need == 0 {
			return nil, fmt.Errorf("llama_tokenize requested zero tokens")
		}
		buf = make([]llamaToken, need)
		n = b.tokenize(vocab, prompt, int32(len(prompt)), &buf[0], int32(len(buf)), true, false)
	}
	if n < 0 {
		return nil, fmt.Errorf("llama_tokenize failed: %d", n)
	}
	return append([]llamaToken(nil), buf[:n]...), nil
}

func (b *llamaCppFFIBackend) tokenPiece(vocab *llamaVocab, token llamaToken) (string, error) {
	buf := make([]byte, 32)
	n := b.tokenToPiece(vocab, token, &buf[0], int32(len(buf)), 0, false)
	if n < 0 {
		need := int(-n)
		if need == 0 {
			return "", fmt.Errorf("llama_token_to_piece requested zero bytes")
		}
		buf = make([]byte, need)
		n = b.tokenToPiece(vocab, token, &buf[0], int32(len(buf)), 0, false)
	}
	if n < 0 {
		return "", fmt.Errorf("llama_token_to_piece failed: %d", n)
	}
	return string(buf[:n]), nil
}

func (b *llamaCppFFIBackend) newSampler(temperature float64) (*llamaSampler, error) {
	if temperature <= 0.01 {
		sampler := b.samplerInitGreedy()
		if sampler == nil {
			return nil, fmt.Errorf("llama_sampler_init_greedy returned nil")
		}
		return sampler, nil
	}

	params := b.samplerChainDefaultParams()
	chain := b.samplerChainInit(params)
	if chain == nil {
		return nil, fmt.Errorf("llama_sampler_chain_init returned nil")
	}

	children := []*llamaSampler{
		b.samplerInitTopK(40),
		b.samplerInitTopP(0.95, 1),
		b.samplerInitTemp(float32(temperature)),
		b.samplerInitDist(uint32(time.Now().UnixNano())),
	}
	for _, child := range children {
		if child == nil {
			b.samplerFree(chain)
			return nil, fmt.Errorf("llama sampler chain child returned nil")
		}
		b.samplerChainAdd(chain, child)
	}
	return chain, nil
}

func fillLlamaBatch(batch *llamaBatch, tokens []llamaToken, posStart int) error {
	if len(tokens) == 0 {
		return fmt.Errorf("empty llama batch")
	}
	if batch == nil {
		return fmt.Errorf("nil llama batch")
	}
	if batch.Token == nil || batch.Pos == nil || batch.NSeqID == nil || batch.SeqID == nil || batch.Logits == nil {
		return fmt.Errorf("llama batch pointers are nil")
	}

	tokenSlice := unsafe.Slice(batch.Token, len(tokens))
	posSlice := unsafe.Slice(batch.Pos, len(tokens))
	nSeqSlice := unsafe.Slice(batch.NSeqID, len(tokens))
	seqPtrs := unsafe.Slice(batch.SeqID, len(tokens))
	logitSlice := unsafe.Slice(batch.Logits, len(tokens))

	batch.NTokens = int32(len(tokens))
	for i, token := range tokens {
		tokenSlice[i] = token
		posSlice[i] = llamaPos(posStart + i)
		nSeqSlice[i] = 1
		if seqPtrs[i] == nil {
			return fmt.Errorf("llama batch seq id pointer is nil at %d", i)
		}
		unsafe.Slice(seqPtrs[i], 1)[0] = 0
		logitSlice[i] = 0
	}
	logitSlice[len(tokens)-1] = 1
	return nil
}

func registerLlamaPuregoFunc(handle uintptr, name string, out any) (err error) {
	sym, err := purego.Dlsym(handle, name)
	if err != nil {
		return fmt.Errorf("lookup %s: %w", name, err)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("register %s: %v", name, recovered)
		}
	}()
	purego.RegisterFunc(out, sym)
	return nil
}

func llamaDirectDefaultThreads() int {
	if runtime.NumCPU() <= 0 {
		return 1
	}
	if runtime.NumCPU() < 4 {
		return runtime.NumCPU()
	}
	return 4
}
