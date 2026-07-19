ALTER TABLE public.user_card_fsrs ADD COLUMN last_rating smallint;
COMMENT ON COLUMN public.user_card_fsrs.last_rating IS
  'FSRS rating (1=Again, 2=Hard, 3=Good, 4=Easy) of the swipe that produced this row''s state. NULL only when the backfill found no swipe history.';

UPDATE public.user_card_fsrs f
SET last_rating = (
  SELECT s.rating
  FROM public.swipe_records s
  WHERE s.user_id = f.user_id AND s.card_id = f.card_id
  ORDER BY s.reviewed_at DESC, s.id DESC
  LIMIT 1
);
