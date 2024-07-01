import React, { useState } from "react";
import { useSwipeable } from "react-swipeable";
import "./SwipeableCard.css";

interface SwipeableCardProps {
  content: string;
  onSwiped: (dir: string) => void;
}

function SwipeableCard({ content, onSwiped }: SwipeableCardProps) {
  const [isFlipped, setIsFlipped] = useState(false);

  const handlers = useSwipeable({
    onSwipedLeft: () => handleSwipe("left"),
    onSwipedRight: () => handleSwipe("right"),
    onSwipedUp: () => handleSwipe("up"),
    onSwipedDown: () => handleSwipe("down"),
  });

  const handleSwipe = (dir: string) => {
    setIsFlipped(true);
    setTimeout(() => {
      onSwiped(dir);
      setIsFlipped(false);
    }, 600); // Match the duration of the CSS transition
  };

  return (
    <div
      {...handlers}
      className={`swipeable-card ${isFlipped ? "flipped" : ""}`}
    >
      <div className="swipeable-card-content">{content}</div>
      <div className="swipeable-card-back">{content} (Back)</div>
    </div>
  );
}

export default SwipeableCard;
