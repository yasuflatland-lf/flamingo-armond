import React, { useState } from "react";
import { useSwipeable } from "react-swipeable";
import { IoIosHeart } from "react-icons/io";
import { AiFillDislike } from "react-icons/ai";
import { GrValidate } from "react-icons/gr";
import "./SwipeableCard.css";

interface SwipeableCardProps {
  content: string;
  onSwiped: (dir: string) => void;
}

function SwipeableCard({ content, onSwiped }: SwipeableCardProps) {
  const [isFlipped, setIsFlipped] = useState(false);
  const [watermark, setWatermark] = useState<React.ReactNode>(null);
  const [watermarkColor, setWatermarkColor] = useState("");
  const [swipeClass, setSwipeClass] = useState("");

  const handlers = useSwipeable({
    onSwipedLeft: () => handleSwipe("left"),
    onSwipedRight: () => handleSwipe("right"),
    onSwipedUp: () => handleSwipe("up"),
    onSwipedDown: () => handleSwipe("down"),
    onSwiping: (eventData) => handleSwiping(eventData),
    onTap: (eventData) => handleTapping(eventData),
  });

  const handleSwipe = (dir: string) => {
    if (dir === "up") {
      setWatermark(null);
      setIsFlipped(true);
    } else {
      setWatermark(null);
      setSwipeClass("");
      onSwiped(dir);
    }
  };

  const handleTapping = ({ dir }: { dir: string }) => {
    setIsFlipped(false);
    console.log(dir);
  };

  const handleSwiping = ({ dir }: { dir: string }) => {
    if (dir === "Left") {
      setSwipeClass("swipe-left");
      setWatermark(<IoIosHeart />);
      setWatermarkColor("text-pink-700 opacity-30");
    } else if (dir === "Right") {
      setSwipeClass("swipe-right");
      setWatermark(<AiFillDislike />);
      setWatermarkColor("text-green-700 opacity-30");
    } else if (dir === "Down") {
      setSwipeClass("");
      setWatermark(<GrValidate />);
      setWatermarkColor("text-green-700 opacity-30");
    } else {
      setSwipeClass("");
      setWatermark(null);
      setWatermarkColor("");
    }
  };

  return (
    <>
      <div
        data-testid="wartermark-id"
        className={`watermark ${watermarkColor}`}
      >
        {watermark}
      </div>
      <div
        {...handlers}
        className={`swipeable-card ${isFlipped ? "flipped" : ""} ${swipeClass} justify-center p-8`}
      >
        <h1 className="font-mono text-4xl font-extrabold swipeable-card-content">
          {content}
        </h1>
        <div className="swipeable-card-back">{content} (Back)</div>
      </div>
    </>
  );
}

export default SwipeableCard;
