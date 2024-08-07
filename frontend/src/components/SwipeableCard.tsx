import React, { useState } from "react";
import { useSwipeable } from "react-swipeable";
import { IoIosHeart } from "react-icons/io";
import { AiFillDislike } from "react-icons/ai";
import { GrValidate } from "react-icons/gr";
import "./SwipeableCard.css";
import { DOWN, LEFT, RIGHT, SwipeEventData, UP } from "react-swipeable/src/types";
import {HandledEvents} from "react-swipeable/src/types.ts";

interface SwipeableCardProps {
  subtitle: string;
  content: string;
  onSwiped: (dir: string) => void;
}

function SwipeableCard({ subtitle, content, onSwiped }: SwipeableCardProps) {
  const [isFlipped, setIsFlipped] = useState(false);
  const [watermark, setWatermark] = useState<React.ReactNode>(null);
  const [watermarkColor, setWatermarkColor] = useState("");
  const [swipeClass, setSwipeClass] = useState("");

  const handlers = useSwipeable({
    onSwiped: (swipeEventData: SwipeEventData) => handleSwipe(swipeEventData),
    onSwiping: (swipeEventData: SwipeEventData) => handleSwiping(swipeEventData),
    onTap: (tapEventData) => handleTapping(tapEventData),
  });

  const handleSwipe = (swipeEventData: SwipeEventData) => {
    if (swipeEventData.dir === UP) {
      setWatermark(null);
      setIsFlipped(true);
    } else {
      setWatermark(null);
      setSwipeClass("");
      onSwiped(swipeEventData.dir);
    }
  };

  const handleTapping = (tapEventData: { event: HandledEvents }) => {
    if(tapEventData.event.isTrusted) {
      setIsFlipped(false);
    }
  };

  const handleSwiping = (eventData: SwipeEventData) => {
    const { dir } = eventData;
    if (dir === LEFT) {
      setSwipeClass("swipe-left");
      setWatermark(<IoIosHeart />);
      setWatermarkColor("text-pink-700 opacity-30");
    } else if (dir === RIGHT) {
      setSwipeClass("swipe-right");
      setWatermark(<AiFillDislike />);
      setWatermarkColor("text-green-700 opacity-30");
    } else if (dir === DOWN) {
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
            data-testid="watermark-id"
            className={`watermark ${watermarkColor}`}
        >
          {watermark}
        </div>
        <div
            {...handlers}
            className={`swipeable-card ${isFlipped ? "flipped" : ""} ${swipeClass} justify-center p-8`}
        >
          <div className="flex flex-col items-start  swipeable-card-content">
            <h4 className="text-gray-400 font-mono text-md whitespace-pre-wrap">
              {subtitle}
            </h4>
            <h1 className="font-mono text-4xl font-extrabold">{content}</h1>
          </div>

          <div className="swipeable-card-back">{content} (Back)</div>
        </div>
      </>
  );
}

export default SwipeableCard;
