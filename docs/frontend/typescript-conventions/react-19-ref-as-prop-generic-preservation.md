# React 19 ref-as-prop preserves the generic parameter that `forwardRef` erases

> Part of [`docs/frontend/typescript-conventions.md`](../typescript-conventions.md). See the index for related rules.

A component that needs to expose an imperative handle to a parent — `triggerSwipe(direction)`, `focus()`, `scrollTo(offset)` — has two encodings under React 19: (a) `forwardRef<H, P>(function Component(props, ref) { ... })`, and (b) accept `ref?: RefObject<H | null>` as an ordinary prop and pass it directly to `useImperativeHandle`. The two encodings look interchangeable until the component is also generic over its data shape, at which point only (b) preserves the generic.

```tsx
// Anti-pattern: forwardRef erases the TCard parameter at the call site.
// TypeScript will infer TCard as the constraint (SwipeCardData) rather than
// the concrete TCard the caller passed in, because forwardRef's type signature
// is not generic-aware: it expects `(props: P, ref: Ref<H>) => ReactElement`
// where P is fixed once the outer function call resolves.
const SwipeCardStack = forwardRef<SwipeCardStackHandle, Props<TCard>>(
  function SwipeCardStack<TCard extends SwipeCardData>(
    { cards, onCardSwiped, ... }: Props<TCard>,
    ref,
  ) {
    // ...
  },
);
// Call site: SwipeCardStack<LearnCard> is not callable as a generic — the
// generic parameter was lost inside forwardRef's typing. Type narrowing on
// `cards` and `onCardSwiped(card, direction)` collapses to SwipeCardData.
```

```tsx
// Correct: accept the ref as a plain prop. The function declaration stays
// generic, so callers can write SwipeCardStack<LearnCard> and have
// `cards: LearnCard[]`, `onCardSwiped: (card: LearnCard, ...) => void` narrow
// to the concrete type.
type SwipeCardStackHandle = {
  triggerSwipe: (direction: SwipeDirection) => void;
};

type Props<TCard extends SwipeCardData> = {
  cards: TCard[];
  onCardSwiped: (card: TCard, direction: SwipeDirection) => void;
  ref?: RefObject<SwipeCardStackHandle | null>;
};

export function SwipeCardStack<TCard extends SwipeCardData>({
  cards,
  onCardSwiped,
  ref,
}: Props<TCard>) {
  // ...
  useImperativeHandle(ref, () => ({ triggerSwipe }), [triggerSwipe]);
  // ...
}
```

**Why:** `forwardRef`'s type signature predates the React 19 ref-as-prop change and is not generic-aware. The wrapper accepts a render function whose props parameter is a concrete shape, so when an inner generic parameter (`TCard`) appears in `Props<TCard>`, the only way TypeScript can satisfy the `forwardRef` signature is to collapse `TCard` to its constraint. The caller writes `SwipeCardStack<LearnCard>` and sees `LearnCard` narrow to `SwipeCardData` inside the component, defeating the point of the parameter. React 19 removed the need for the wrapper: a ref passed as an ordinary prop reaches `useImperativeHandle` with the same semantics, and the function declaration stays a normal generic function whose type system sees the parameter throughout.

**How to apply:** any component that (a) expects to expose an imperative handle AND (b) is generic over its data shape should accept the ref via a plain prop. Type the prop as `ref?: RefObject<H | null>` where `H` is the handle type — the `| null` matches `useRef<H>(null)`'s default initial value at the call site. Pass the prop directly to `useImperativeHandle(ref, factory, deps)`; no destructuring or null-guard is needed because `useImperativeHandle` accepts an undefined ref. Components that have no generic parameter may still use `forwardRef`, but the codebase prefers ref-as-prop for new code because the two encodings are no longer worth distinguishing — uniformity is cheaper than the wrapper. Reference: `SwipeCardStack` in `frontend/src/components/learn/swipe-card-stack.tsx`.

## Stabilize the imperative handle with `useRef` mirrors for fast-changing callbacks

`useImperativeHandle(ref, factory, deps)` re-invokes `factory` whenever any dep changes, replacing the object the parent's ref points at. If the factory closes over a parent callback that gets a fresh identity each render (e.g. `onCardSwiped` rebuilt when the parent's state changes), the handle's `triggerSwipe` reference also changes — any consumer that depends on handle stability for a `useEffect` cleanup or a stale-closure assumption will misbehave. Mirror the callback into a ref and read `ref.current` inside the handle's methods:

```tsx
// Mirror the parent-provided callback into a ref so the handle stays stable.
const onCardSwipedRef = useRef(onCardSwiped);
useEffect(() => {
  onCardSwipedRef.current = onCardSwiped;
}, [onCardSwiped]);

const triggerSwipe = useCallback((direction: SwipeDirection) => {
  const card = activeCardRef.current;
  if (!card) return;
  onCardSwipedRef.current(card, direction);
}, []); // empty deps — the callback identity is anchored

useImperativeHandle(ref, () => ({ triggerSwipe }), [triggerSwipe]);
```

The mirror is the same shape covered in [§ "Stabilize callback identity via `useRef` mirrors when the callback reads frequently-changing state"](./stabilize-callback-identity-via-useref-mirrors.md); the new ground here is that `useImperativeHandle`'s `deps` array participates in the same identity chain. A non-empty `deps` array that lists a re-created callback rebuilds the handle every render, so the deps must transitively bottom out at stable references — either an empty array (closing over refs only) or a list of `useCallback` results that themselves close over refs.

**Pitfall — `useImperativeHandle` deps lint:** Biome's `useExhaustiveDependencies` will flag any closure variable that is not in the deps array. The fix is **not** to silence the lint; it is to make every closure variable either a ref read (`ref.current`) or a `useCallback` whose identity is stable. The lint pointing at an "unstable" dep is the early warning that the handle will rebuild on every render — fix the underlying instability instead of adding the dep and accepting the churn. Reference: the `triggerSwipe` / `useImperativeHandle` pair in `SwipeCardStack` closes only over refs, so the `useCallback` and `useImperativeHandle` deps are minimal without lint suppression.

**Trade-off — when `forwardRef` is still the right choice:** a non-generic component that does nothing inside `useImperativeHandle` beyond exposing the underlying DOM ref (`useImperativeHandle(ref, () => inputElementRef.current!)`) is conventionally written with `forwardRef`; the ref-as-prop form here has no advantage and reads as a non-idiomatic deviation. The deciding axis is "does the component have a generic parameter whose narrowing the caller depends on?" — if yes, ref-as-prop; if no, either form is acceptable and the codebase tolerates both.
